/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scope

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type (
	// MachinePoolScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolScopeParams struct {
		Client         client.Client
		ClusterGetter  cloud.ClusterGetter
		MachinePool    *clusterv1exp.MachinePool
		GCPMachinePool *infrav1exp.GCPMachinePool
	}
	// MachinePoolScope defines a scope defined around a machine pool and its cluster.
	MachinePoolScope struct {
		Client                     client.Client
		PatchHelper                *patch.Helper
		CapiMachinePoolPatchHelper *patch.Helper

		ClusterGetter  cloud.ClusterGetter
		MachinePool    *clusterv1exp.MachinePool
		GCPMachinePool *infrav1exp.GCPMachinePool
	}
)

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	if params.GCPMachinePool == nil {
		return nil, errors.New("gcp machine pool is required when creating a MachinePoolScope")
	}

	helper, err := patch.NewHelper(params.GCPMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", params.GCPMachinePool.GroupVersionKind(), params.GCPMachinePool.Namespace, params.GCPMachinePool.Name)
	}

	capiMachinePoolPatchHelper, err := patch.NewHelper(params.MachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", params.MachinePool.GroupVersionKind(), params.MachinePool.Namespace, params.MachinePool.Name)
	}

	return &MachinePoolScope{
		Client:                     params.Client,
		ClusterGetter:              params.ClusterGetter,
		MachinePool:                params.MachinePool,
		GCPMachinePool:             params.GCPMachinePool,
		PatchHelper:                helper,
		CapiMachinePoolPatchHelper: capiMachinePoolPatchHelper,
	}, nil
}

// SetReady sets the GCPMachinePool Ready Status to true.
func (m *MachinePoolScope) SetReady() {
	m.GCPMachinePool.Status.Ready = true
}

// SetNotReady sets the GCPMachinePool Ready Status to false.
func (m *MachinePoolScope) SetNotReady() {
	m.GCPMachinePool.Status.Ready = false
}

// SetFailureMessage sets the GCPMachinePool status failure message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.GCPMachinePool.Status.FailureMessage = ptr.To(v.Error())
}

// SetFailureReason sets the GCPMachinePool status failure reason.
func (m *MachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.GCPMachinePool.Status.FailureReason = &v
}

// PatchObject persists the GCPMachinePool spec and status on the API server.
func (m *MachinePoolScope) PatchObject(ctx context.Context) error {
	return m.PatchHelper.Patch(
		ctx,
		m.GCPMachinePool,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1exp.GCPMachinePoolReadyCondition,
			infrav1exp.GCPMachinePoolCreatingCondition,
			infrav1exp.GCPMachinePoolDeletingCondition,
		}},
	)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachinePoolScope) Close(ctx context.Context) error {
	if err := m.PatchObject(ctx); err != nil {
		return err
	}
	if err := m.PatchCAPIMachinePoolObject(ctx); err != nil {
		return errors.Wrap(err, "unable to patch CAPI MachinePool")
	}

	return nil
}

// InstanceGroupTemplateBuilder returns a GCP instance template.
func (m *MachinePoolScope) InstanceGroupTemplateBuilder(bootstrapData string) *compute.InstanceTemplate {
	instanceTemplate := &compute.InstanceTemplate{
		Name: m.GCPMachinePool.Name,
		Properties: &compute.InstanceProperties{
			MachineType:    m.GCPMachinePool.Spec.InstanceType,
			MinCpuPlatform: m.MinCPUPlatform(),
			Tags: &compute.Tags{
				Items: append(
					m.GCPMachinePool.Spec.AdditionalNetworkTags,
					fmt.Sprintf("%s-%s", m.ClusterGetter.Name(), m.Role()),
					m.ClusterGetter.Name(),
				),
			},
			Labels: infrav1.Build(infrav1.BuildParams{
				ClusterName: m.ClusterGetter.Name(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        ptr.To(m.Role()),
				Additional:  m.ClusterGetter.AdditionalLabels().AddLabels(m.GCPMachinePool.Spec.AdditionalLabels),
			}),
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					{
						Key:   "user-data",
						Value: ptr.To(bootstrapData),
					},
				},
			},
		},
	}

	instanceTemplate.Properties.Disks = append(instanceTemplate.Properties.Disks, m.InstanceImageSpec())
	instanceTemplate.Properties.Disks = append(instanceTemplate.Properties.Disks, m.InstanceAdditionalDiskSpec()...)
	instanceTemplate.Properties.ServiceAccounts = append(instanceTemplate.Properties.ServiceAccounts, m.InstanceServiceAccountsSpec())
	instanceTemplate.Properties.NetworkInterfaces = append(instanceTemplate.Properties.NetworkInterfaces, m.InstanceNetworkInterfaceSpec())

	return instanceTemplate
}

// InstanceNetworkInterfaceSpec returns the network interface spec for the instance.
func (m *MachinePoolScope) InstanceNetworkInterfaceSpec() *compute.NetworkInterface {
	networkInterface := &compute.NetworkInterface{
		Network: path.Join("projects", m.ClusterGetter.Project(), "global", "networks", m.ClusterGetter.NetworkName()),
	}

	if m.GCPMachinePool.Spec.PublicIP != nil && *m.GCPMachinePool.Spec.PublicIP {
		networkInterface.AccessConfigs = []*compute.AccessConfig{
			{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		}
	}

	if m.GCPMachinePool.Spec.Subnet != nil {
		networkInterface.Subnetwork = path.Join("regions", m.ClusterGetter.Region(), "subnetworks", *m.GCPMachinePool.Spec.Subnet)
	}

	return networkInterface
}

// InstanceAdditionalMetadataSpec returns the additional metadata for the instance.
func (m *MachinePoolScope) InstanceAdditionalMetadataSpec() *compute.MetadataItems {
	metadataItems := new(compute.MetadataItems)
	for _, additionalMetadata := range m.GCPMachinePool.Spec.AdditionalMetadata {
		metadataItems = &compute.MetadataItems{
			Key:   additionalMetadata.Key,
			Value: additionalMetadata.Value,
		}
	}
	return metadataItems
}

// InstanceAdditionalDiskSpec returns the additional disks for the instance.
func (m *MachinePoolScope) InstanceAdditionalDiskSpec() []*compute.AttachedDisk {
	additionalDisks := make([]*compute.AttachedDisk, 0, len(m.GCPMachinePool.Spec.AdditionalDisks))

	for _, disk := range m.GCPMachinePool.Spec.AdditionalDisks {
		additionalDisk := &compute.AttachedDisk{
			AutoDelete: true,
			InitializeParams: &compute.AttachedDiskInitializeParams{
				DiskSizeGb: ptr.Deref(disk.Size, 30),
				DiskType:   string(*disk.DeviceType),
			},
		}
		if strings.HasSuffix(additionalDisk.InitializeParams.DiskType, string(infrav1.LocalSsdDiskType)) {
			additionalDisk.Type = "SCRATCH" // Default is PERSISTENT.
			// Override the Disk size
			additionalDisk.InitializeParams.DiskSizeGb = 375
			// For local SSDs set interface to NVME (instead of default SCSI) which is faster.
			// Most OS images would work with both NVME and SCSI disks but some may work
			// considerably faster with NVME.
			// https://cloud.google.com/compute/docs/disks/local-ssd#choose_an_interface
			additionalDisk.Interface = "NVME"
		}
		additionalDisks = append(additionalDisks, additionalDisk)
	}
	return additionalDisks
}

// InstanceImageSpec returns the image spec for the instance.
func (m *MachinePoolScope) InstanceImageSpec() *compute.AttachedDisk {
	version := ""
	if m.MachinePool.Spec.Template.Spec.Version != nil {
		version = *m.MachinePool.Spec.Template.Spec.Version
	}
	image := "capi-ubuntu-1804-k8s-" + strings.ReplaceAll(semver.MajorMinor(version), ".", "-")
	sourceImage := path.Join("projects", m.ClusterGetter.Project(), "global", "images", "family", image)
	if m.GCPMachinePool.Spec.Image != nil {
		sourceImage = *m.GCPMachinePool.Spec.Image
	} else if m.GCPMachinePool.Spec.ImageFamily != nil {
		sourceImage = *m.GCPMachinePool.Spec.ImageFamily
	}

	diskType := infrav1exp.PdStandardDiskType
	if t := m.GCPMachinePool.Spec.RootDeviceType; t != nil {
		diskType = *t
	}

	return &compute.AttachedDisk{
		AutoDelete: true,
		Boot:       true,
		InitializeParams: &compute.AttachedDiskInitializeParams{
			DiskSizeGb:  m.GCPMachinePool.Spec.RootDeviceSize,
			DiskType:    string(diskType),
			SourceImage: sourceImage,
		},
	}
}

// MinCPUPlatform returns the min cpu platform for the machine pool.
func (m *MachinePoolScope) MinCPUPlatform() string {
	// map of machine types to their respective processors (e2 cannot have a min cpu platform set, so it is not included here)
	var processors = map[string]string{
		"n1":  "Intel Skylake",
		"n2":  "Intel Ice Lake",
		"n2d": "AMD Milan",
		"c3":  "Intel Sapphire Rapids",
		"c2":  "Intel Cascade Lake",
		"t2d": "AMD Milan",
		"m1":  "Intel Skylake",
	}

	// If the min cpu platform is set on the GCPMachinePool, use the specified value.
	if m.GCPMachinePool.Spec.MinCPUPlatform != nil {
		return *m.GCPMachinePool.Spec.MinCPUPlatform
	}

	// If the min cpu platform is not set on the GCPMachinePool, use the default value for the machine type.
	for machineType, processor := range processors {
		if strings.HasPrefix(m.GCPMachinePool.Spec.InstanceType, machineType) {
			return processor
		}
	}

	// If the machine type is not recognized, return an empty string (This will hand off the processor selection to GCP).
	return ""
}

// Zone returns the zone for the machine pool.
func (m *MachinePoolScope) Zone() string {
	if m.MachinePool.Spec.Template.Spec.FailureDomain == nil {
		fd := m.ClusterGetter.FailureDomains()
		if len(fd) == 0 {
			return ""
		}
		zones := make([]string, 0, len(fd))
		for zone := range fd {
			zones = append(zones, zone)
		}
		sort.Strings(zones)
		return zones[0]
	}
	return *m.MachinePool.Spec.Template.Spec.FailureDomain
}

// Role returns the machine role from the labels.
func (m *MachinePoolScope) Role() string {
	return "node"
}

// InstanceServiceAccountsSpec returns service-account spec.
func (m *MachinePoolScope) InstanceServiceAccountsSpec() *compute.ServiceAccount {
	serviceAccount := &compute.ServiceAccount{
		Email: "default",
		Scopes: []string{
			compute.CloudPlatformScope,
		},
	}

	if m.GCPMachinePool.Spec.ServiceAccount != nil {
		serviceAccount.Email = m.GCPMachinePool.Spec.ServiceAccount.Email
		serviceAccount.Scopes = m.GCPMachinePool.Spec.ServiceAccount.Scopes
	}

	return serviceAccount
}

// InstanceGroupBuilder returns an instance group manager spec.
func (m *MachinePoolScope) InstanceGroupBuilder(instanceTemplateName string) *compute.InstanceGroupManager {
	return &compute.InstanceGroupManager{
		Name:             m.GCPMachinePool.Name,
		BaseInstanceName: m.GCPMachinePool.Name,
		InstanceTemplate: path.Join("projects", m.ClusterGetter.Project(), "global", "instanceTemplates", instanceTemplateName),
		TargetSize:       int64(*m.MachinePool.Spec.Replicas),
	}
}

// Project return the project for the GCPMachinePool's cluster.
func (m *MachinePoolScope) Project() string {
	return m.ClusterGetter.Project()
}

// GetGCPClientCredentials returns the GCP client credentials.
func (m *MachinePoolScope) GetGCPClientCredentials() ([]byte, error) {
	credsPath := os.Getenv(ConfigFileEnvVar)
	if credsPath == "" {
		return nil, fmt.Errorf("no ADC environment variable found for credentials (expect %s)", ConfigFileEnvVar)
	}

	byteValue, err := os.ReadFile(credsPath) //nolint:gosec // We need to read a file here
	if err != nil {
		return nil, fmt.Errorf("reading credentials from file %s: %w", credsPath, err)
	}
	return byteValue, nil
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachinePoolScope) GetBootstrapData() (string, error) {
	if m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName}
	if err := m.Client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for GCPMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return string(value), nil
}

// GetInstanceTemplateHash returns the hash of the instance template. The hash is used to identify the instance template.
func (m *MachinePoolScope) GetInstanceTemplateHash(instance *compute.InstanceTemplate) (string, error) {
	instanceBytes, err := json.Marshal(instance)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(instanceBytes)
	shortHash := hash[:4]
	return fmt.Sprintf("%08x", shortHash), nil
}

// SetAnnotation sets a key value annotation on the GCPMachinePool.
func (m *MachinePoolScope) SetAnnotation(key, value string) {
	if m.GCPMachinePool.Annotations == nil {
		m.GCPMachinePool.Annotations = map[string]string{}
	}
	m.GCPMachinePool.Annotations[key] = value
}

// Namespace returns the GCPMachine namespace.
func (m *MachinePoolScope) Namespace() string {
	return m.MachinePool.Namespace
}

// Name returns the GCPMachine name.
func (m *MachinePoolScope) Name() string {
	return m.GCPMachinePool.Name
}

// HasReplicasExternallyManaged returns true if the machine pool has replicas externally managed.
func (m *MachinePoolScope) HasReplicasExternallyManaged(_ context.Context) bool {
	return annotations.ReplicasManagedByExternalAutoscaler(m.MachinePool)
}

// PatchCAPIMachinePoolObject persists the capi machinepool configuration and status.
func (m *MachinePoolScope) PatchCAPIMachinePoolObject(ctx context.Context) error {
	return m.CapiMachinePoolPatchHelper.Patch(
		ctx,
		m.MachinePool,
	)
}

// UpdateCAPIMachinePoolReplicas updates the associated MachinePool replica count.
func (m *MachinePoolScope) UpdateCAPIMachinePoolReplicas(_ context.Context, replicas *int32) {
	m.MachinePool.Spec.Replicas = replicas
}

// ReconcileReplicas ensures MachinePool replicas match MIG capacity if replicas are externally managed by an autoscaler.
func (m *MachinePoolScope) ReconcileReplicas(ctx context.Context, mig *compute.InstanceGroupManager) error {
	log := log.FromContext(ctx)

	if !m.HasReplicasExternallyManaged(ctx) {
		return nil
	}
	log.Info("Replicas are externally managed, skipping replica reconciliation", "machinepool", m.MachinePool.Name)

	var replicas int32
	if m.MachinePool.Spec.Replicas != nil {
		replicas = *m.MachinePool.Spec.Replicas
	}

	if capacity := int32(mig.TargetSize); capacity != replicas {
		m.UpdateCAPIMachinePoolReplicas(ctx, &capacity)
	}

	return nil
}

// SetReplicas sets the machine pool replicas.
func (m *MachinePoolScope) SetReplicas(replicas int32) {
	m.MachinePool.Spec.Replicas = &replicas
}

// ConditionSetter returns the condition setter for the GCPMachinePool.
func (m *MachinePoolScope) ConditionSetter() conditions.Setter {
	return m.GCPMachinePool
}
