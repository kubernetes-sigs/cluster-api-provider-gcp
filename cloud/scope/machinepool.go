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
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	machinepool "sigs.k8s.io/cluster-api-provider-gcp/cloud/scope/strategies/machinepool_deployments"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/processors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/labels/format"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		ClusterGetter              cloud.ClusterGetter
		MachinePool                *clusterv1exp.MachinePool
		GCPMachinePool             *infrav1exp.GCPMachinePool
		migState                   *compute.InstanceGroupManager
		migInstances               []*compute.ManagedInstance
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

// SetMIGState updates the machine pool scope with the current state of the MIG.
func (m *MachinePoolScope) SetMIGState(migState *compute.InstanceGroupManager) {
	m.migState = migState
}

// SetMIGInstances updates the machine pool scope with the current state of the MIG instances.
func (m *MachinePoolScope) SetMIGInstances(migInstances []*compute.ManagedInstance) {
	m.migInstances = migInstances
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
	if m.migState != nil && m.migInstances != nil {
		if err := m.applyGCPMachinePoolMachines(ctx); err != nil {
			return errors.Wrap(err, "failed to apply GCPMachinePoolMachines")
		}

		m.setProvisioningStateAndConditions()
		if err := m.updateReplicasAndProviderIDs(ctx); err != nil {
			return errors.Wrap(err, "failed to update replicas and providerIDs")
		}
	}

	if err := m.PatchObject(ctx); err != nil {
		return errors.Wrap(err, "failed to patch GCPMachinePool")
	}
	if err := m.PatchCAPIMachinePoolObject(ctx); err != nil {
		return errors.Wrap(err, "unable to patch CAPI MachinePool")
	}

	return nil
}

// updateReplicasAndProviderIDs updates the GCPMachinePool replicas and providerIDs.
func (m *MachinePoolScope) updateReplicasAndProviderIDs(ctx context.Context) error {
	machines, err := m.GetMachinePoolMachines(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get machine pool machines")
	}

	var readyReplicas int32
	providerIDs := make([]string, len(machines))
	for i, machine := range machines {
		if machine.Status.Ready {
			readyReplicas++
		}
		providerIDs[i] = machine.Spec.ProviderID
	}

	m.GCPMachinePool.Status.Replicas = readyReplicas
	m.GCPMachinePool.Spec.ProviderIDList = providerIDs
	m.MachinePool.Spec.ProviderIDList = providerIDs
	m.MachinePool.Status.Replicas = readyReplicas
	return nil
}

// setProvisioningStateAndConditions sets the GCPMachinePool provisioning state and conditions.
func (m *MachinePoolScope) setProvisioningStateAndConditions() {
	switch {
	case *m.MachinePool.Spec.Replicas == m.GCPMachinePool.Status.Replicas:
		// MIG is provisioned with enough ready replicas
		m.SetReady()
		conditions.MarkTrue(m.ConditionSetter(), infrav1exp.GCPMachinePoolReadyCondition)
		conditions.MarkFalse(m.ConditionSetter(), infrav1exp.GCPMachinePoolCreatingCondition, infrav1exp.GCPMachinePoolUpdatedReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkFalse(m.ConditionSetter(), infrav1exp.GCPMachinePoolUpdatingCondition, infrav1exp.GCPMachinePoolUpdatedReason, clusterv1.ConditionSeverityInfo, "")
	case *m.MachinePool.Spec.Replicas != m.GCPMachinePool.Status.Replicas:
		// MIG is still provisioning
		m.SetNotReady()
		conditions.MarkFalse(m.ConditionSetter(), infrav1exp.GCPMachinePoolReadyCondition, infrav1exp.GCPMachinePoolCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(m.ConditionSetter(), infrav1exp.GCPMachinePoolUpdatingCondition)
	default:
		m.SetNotReady()
		conditions.MarkFalse(m.ConditionSetter(), infrav1exp.GCPMachinePoolReadyCondition, infrav1exp.GCPMachinePoolCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(m.ConditionSetter(), infrav1exp.GCPMachinePoolUpdatingCondition)
	}
}

func (m *MachinePoolScope) applyGCPMachinePoolMachines(ctx context.Context) error {
	log := log.FromContext(ctx)

	if m.migState == nil {
		return nil
	}

	gmpms, err := m.GetMachinePoolMachines(ctx)
	if err != nil {
		return err
	}

	existingMachinesByProviderID := make(map[string]infrav1exp.GCPMachinePoolMachine, len(gmpms))
	for _, machine := range gmpms {
		existingMachinesByProviderID[machine.Spec.ProviderID] = machine
	}

	gcpMachinesByProviderID := m.InstancesByProviderID()
	for key, val := range gcpMachinesByProviderID {
		if _, ok := existingMachinesByProviderID[key]; !ok {
			log.Info("Creating GCPMachinePoolMachine", "machine", val.Name, "providerID", key)
			if err := m.createMachine(ctx, val); err != nil {
				return errors.Wrap(err, "failed creating GCPMachinePoolMachine")
			}
			continue
		}
	}

	deleted := false
	// delete machines that no longer exist in GCP
	for key, machine := range existingMachinesByProviderID {
		machine := machine
		if _, ok := gcpMachinesByProviderID[key]; !ok {
			deleted = true
			log.V(4).Info("deleting GCPMachinePoolMachine because it no longer exists in the MIG", "providerID", key)
			delete(existingMachinesByProviderID, key)
			if err := m.Client.Delete(ctx, &machine); err != nil {
				return errors.Wrap(err, "failed deleting GCPMachinePoolMachine no longer existing in GCP")
			}
		}
	}

	if deleted {
		log.Info("GCPMachinePoolMachines deleted, requeueing")
		return nil
	}

	// when replicas are externally managed, we do not want to scale down manually since that is handled by the external scaler.
	if m.HasReplicasExternallyManaged(ctx) {
		log.Info("Replicas are externally managed, skipping scaling down")
		return nil
	}

	deleteSelector := m.getDeploymentStrategy()
	if deleteSelector == nil {
		log.V(4).Info("can not select GCPMachinePoolMachines to delete because no deployment strategy is specified")
		return nil
	}

	// select machines to delete to lower the replica count
	toDelete, err := deleteSelector.SelectMachinesToDelete(ctx, m.DesiredReplicas(), existingMachinesByProviderID)
	if err != nil {
		return errors.Wrap(err, "failed selecting GCPMachinePoolMachines to delete")
	}

	for _, machine := range toDelete {
		machine := machine
		log.Info("deleting selected GCPMachinePoolMachine", "providerID", machine.Spec.ProviderID)
		if err := m.Client.Delete(ctx, &machine); err != nil {
			return errors.Wrap(err, "failed deleting GCPMachinePoolMachine to reduce replica count")
		}
	}
	return nil
}

func (m *MachinePoolScope) createMachine(ctx context.Context, managedInstance compute.ManagedInstance) error {
	gmpm := infrav1exp.GCPMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedInstance.Name,
			Namespace: m.GCPMachinePool.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1exp.GroupVersion.String(),
					Kind:               "GCPMachinePool",
					Name:               m.GCPMachinePool.Name,
					BlockOwnerDeletion: ptr.To(true),
					UID:                m.GCPMachinePool.UID,
				},
			},
			Labels: map[string]string{
				m.ClusterGetter.Name():          string(infrav1.ResourceLifecycleOwned),
				clusterv1.ClusterNameLabel:      m.ClusterGetter.Name(),
				infrav1exp.MachinePoolNameLabel: m.GCPMachinePool.Name,
				clusterv1.MachinePoolNameLabel:  format.MustFormatValue(m.MachinePool.Name),
			},
		},
		Spec: infrav1exp.GCPMachinePoolMachineSpec{
			ProviderID: m.ProviderIDInstance(&managedInstance),
			InstanceID: strconv.FormatUint(managedInstance.Id, 10),
		},
	}

	controllerutil.AddFinalizer(&gmpm, infrav1exp.GCPMachinePoolMachineFinalizer)
	if err := m.Client.Create(ctx, &gmpm); err != nil {
		return errors.Wrapf(err, "failed creating GCPMachinePoolMachine %s in GCPMachinePool %s", managedInstance.Name, m.GCPMachinePool.Name)
	}

	return nil
}

func (m *MachinePoolScope) getDeploymentStrategy() machinepool.TypedDeleteSelector {
	if m.GCPMachinePool == nil {
		return nil
	}

	return machinepool.NewMachinePoolDeploymentStrategy(m.GCPMachinePool.Spec.Strategy)
}

// GetMachinePoolMachines returns the list of GCPMachinePoolMachines associated with this GCPMachinePool.
func (m *MachinePoolScope) GetMachinePoolMachines(ctx context.Context) ([]infrav1exp.GCPMachinePoolMachine, error) {
	labels := m.getMachinePoolMachineLabels()
	gmpml := &infrav1exp.GCPMachinePoolMachineList{}
	if err := m.Client.List(ctx, gmpml, client.InNamespace(m.GCPMachinePool.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrap(err, "failed to list GCPMachinePoolMachines")
	}

	return gmpml.Items, nil
}

// DesiredReplicas returns the replica count on machine pool or 0 if machine pool replicas is nil.
func (m MachinePoolScope) DesiredReplicas() int32 {
	return ptr.Deref(m.MachinePool.Spec.Replicas, 0)
}

// InstancesByProviderID returns a map of GCPMachinePoolMachine instances by providerID.
func (m *MachinePoolScope) InstancesByProviderID() map[string]compute.ManagedInstance {
	instances := make(map[string]compute.ManagedInstance, len(m.migInstances))
	for _, instance := range m.migInstances {
		if instance.InstanceStatus == "RUNNING" && instance.CurrentAction == "NONE" || instance.InstanceStatus == "PROVISIONING" {
			instances[m.ProviderIDInstance(instance)] = *instance
		}
	}
	return instances
}

func (m *MachinePoolScope) getMachinePoolMachineLabels() map[string]string {
	return map[string]string{
		clusterv1.ClusterNameLabel:      m.ClusterGetter.Name(),
		infrav1exp.MachinePoolNameLabel: m.GCPMachinePool.Name,
		clusterv1.MachinePoolNameLabel:  format.MustFormatValue(m.MachinePool.Name),
		m.ClusterGetter.Name():          string(infrav1.ResourceLifecycleOwned),
	}
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
	instanceTemplate.Properties.Metadata.Items = append(instanceTemplate.Properties.Metadata.Items, m.InstanceAdditionalMetadataSpec()...)
	instanceTemplate.Properties.ShieldedInstanceConfig = m.GetShieldedInstanceConfigSpec()

	return instanceTemplate
}

// GetShieldedInstanceConfigSpec returns the shielded config spec for the instance
// As of now will only build the configuration for using
// - Integrity Monitoring - enabled by default
// - Secure Boot - disabled by default
// - vTPM - enabled by default.
func (m *MachinePoolScope) GetShieldedInstanceConfigSpec() *compute.ShieldedInstanceConfig {
	shieldedInstanceConfig := &compute.ShieldedInstanceConfig{
		EnableSecureBoot:          false,
		EnableVtpm:                true,
		EnableIntegrityMonitoring: true,
	}
	if m.GCPMachinePool.Spec.ShieldedInstanceConfig != nil {
		if m.GCPMachinePool.Spec.ShieldedInstanceConfig.SecureBoot == infrav1exp.SecureBootPolicyEnabled {
			shieldedInstanceConfig.EnableSecureBoot = true
		}
		if m.GCPMachinePool.Spec.ShieldedInstanceConfig.VirtualizedTrustedPlatformModule == infrav1exp.VirtualizedTrustedPlatformModulePolicyDisabled {
			shieldedInstanceConfig.EnableVtpm = false
		}
		if m.GCPMachinePool.Spec.ShieldedInstanceConfig.IntegrityMonitoring == infrav1exp.IntegrityMonitoringPolicyDisabled {
			shieldedInstanceConfig.EnableIntegrityMonitoring = false
		}
	}
	return shieldedInstanceConfig
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
func (m *MachinePoolScope) InstanceAdditionalMetadataSpec() []*compute.MetadataItems {
	metadataItems := make([]*compute.MetadataItems, 0, len(m.GCPMachinePool.Spec.AdditionalMetadata))

	for _, additionalMetadata := range m.GCPMachinePool.Spec.AdditionalMetadata {
		metadataItems = append(metadataItems, &compute.MetadataItems{
			Key:   additionalMetadata.Key,
			Value: additionalMetadata.Value,
		})
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
				DiskType:   *disk.DeviceType,
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
	image := cloud.ClusterAPIImagePrefix + strings.ReplaceAll(semver.MajorMinor(version), ".", "-")
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
	// If the min cpu platform is set on the GCPMachinePool, use the specified value.
	if m.GCPMachinePool.Spec.MinCPUPlatform != nil {
		return *m.GCPMachinePool.Spec.MinCPUPlatform
	}

	// Return the latest processor for the instance type or empty string if unknown instance type
	return processors.GetLatestProcessor(m.GCPMachinePool.Spec.InstanceType)
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
		TargetSize:       int64(m.DesiredReplicas()),
	}
}

// InstanceGroupUpdate returns an instance group manager spec.
func (m *MachinePoolScope) InstanceGroupUpdate(instanceTemplateName string) *compute.InstanceGroupManager {
	return &compute.InstanceGroupManager{
		Name:             m.GCPMachinePool.Name,
		BaseInstanceName: m.GCPMachinePool.Name,
		InstanceTemplate: path.Join("projects", m.ClusterGetter.Project(), "global", "instanceTemplates", instanceTemplateName),
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

// NeedsRequeue returns true if the machine pool needs to be requeued.
func (m *MachinePoolScope) NeedsRequeue() bool {
	numberOfRunningInstances := 0
	for _, instance := range m.migInstances {
		if instance.InstanceStatus == "RUNNING" {
			numberOfRunningInstances++
		}
	}

	return numberOfRunningInstances != int(m.DesiredReplicas())
}

// SetAnnotation sets a key value annotation on the GCPMachinePool.
func (m *MachinePoolScope) SetAnnotation(key, value string) {
	if m.GCPMachinePool.Annotations == nil {
		m.GCPMachinePool.Annotations = map[string]string{}
	}
	m.GCPMachinePool.Annotations[key] = value
}

// Namespace returns the GCPMachinePool namespace.
func (m *MachinePoolScope) Namespace() string {
	return m.MachinePool.Namespace
}

// Name returns the GCPMachinePool name.
func (m *MachinePoolScope) Name() string {
	return m.GCPMachinePool.Name
}

// ProviderIDInstance returns the GCPMachinePool providerID for a managed instance.
func (m *MachinePoolScope) ProviderIDInstance(managedInstance *compute.ManagedInstance) string {
	return fmt.Sprintf("gce://%s/%s/%s", m.Project(), m.GCPMachinePool.Spec.Zone, managedInstance.Name)
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

// ReconcileReplicas ensures MachinePool replicas match MIG capacity unless replicas are externally managed by an autoscaler.
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
