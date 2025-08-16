/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"
	"path"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// ekscontrolplanev1 "sigs.k8s.io/cluster-api-provider-gcp/controlplane/eks/api/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/shared"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/gcp"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/logger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// MachinePoolScope defines a scope defined around a machine and its cluster.
type MachinePoolScope struct {
	// InstanceSpecBuilder

	// logger.Logger
	client                     client.Client
	patchHelper                *patch.Helper
	capiMachinePoolPatchHelper *patch.Helper

	ClusterGetter cloud.ClusterGetter
	// Cluster       *clusterv1.Cluster
	MachinePool *expclusterv1.MachinePool
	// InfraCluster   EC2Scope
	GCPMachinePool *expinfrav1.GCPMachinePool
}

// MachinePoolScopeParams defines a scope defined around a machine and its cluster.
type MachinePoolScopeParams struct {
	client.Client
	// Logger *logger.Logger

	ClusterGetter cloud.ClusterGetter
	// Cluster       *clusterv1.Cluster
	MachinePool *expclusterv1.MachinePool
	// InfraCluster   EC2Scope
	GCPMachinePool *expinfrav1.GCPMachinePool
}

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.ClusterGetter == nil {
		return nil, errors.New("clusterGetter is required when creating a MachinePoolScope")
	}
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machinepool is required when creating a MachinePoolScope")
	}
	// if params.Cluster == nil {
	// 	return nil, errors.New("cluster is required when creating a MachinePoolScope")
	// }
	if params.GCPMachinePool == nil {
		return nil, errors.New("gcp machine pool is required when creating a MachinePoolScope")
	}
	// if params.InfraCluster == nil {
	// 	return nil, errors.New("gcp cluster is required when creating a MachinePoolScope")
	// }

	// if params.Logger == nil {
	// 	log := klog.Background()
	// 	params.Logger = logger.NewLogger(log)
	// }

	ampHelper, err := patch.NewHelper(params.GCPMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init GCPMachinePool patch helper")
	}
	mpHelper, err := patch.NewHelper(params.MachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init MachinePool patch helper")
	}

	// machinePoolSpec := params.GCPMachinePool.Spec
	// gcpMachineSpec := &infrav1.GCPMachineSpec{
	// 	InstanceType: machinePoolSpec.InstanceType,
	// }

	// zones := params.MachinePool.Spec.FailureDomains

	return &MachinePoolScope{
		// InstanceSpecBuilder: InstanceSpecBuilder{
		// 	// zones:   zones,
		// 	GCPMachineSpec: gcpMachineSpec,
		// 	Version:        nil, // TODO: Do we need this?  params.MachinePool.Spec.Template.Spec.Version,
		// },
		// Logger:                     *params.Logger,
		client:                     params.Client,
		patchHelper:                ampHelper,
		capiMachinePoolPatchHelper: mpHelper,

		ClusterGetter: params.ClusterGetter,
		// Cluster:       params.Cluster,
		MachinePool: params.MachinePool,
		// InfraCluster:   params.InfraCluster,
		GCPMachinePool: params.GCPMachinePool,
	}, nil
}

// Cloud returns initialized cloud.
func (m *MachinePoolScope) Cloud() cloud.Cloud {
	return m.ClusterGetter.Cloud()
}

// // GetProviderID returns the GCPMachine providerID from the spec.
// func (m *MachinePoolScope) GetProviderID() string {
// 	if m.GCPMachinePool.Spec.ProviderID != "" {
// 		return m.GCPMachinePool.Spec.ProviderID
// 	}
// 	return ""
// }

// // Ignition gets the ignition config.
// func (m *MachinePoolScope) Ignition() *infrav1.Ignition {
// 	return m.GCPMachinePool.Spec.Ignition
// }

// Name returns the GCPMachinePool name.
func (m *MachinePoolScope) Name() string {
	return m.GCPMachinePool.Name
}

// Namespace returns the namespace name.
func (m *MachinePoolScope) Namespace() string {
	return m.GCPMachinePool.Namespace
}

// // GetRawBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName,
// // including the secret's namespaced name.
// func (m *MachinePoolScope) GetRawBootstrapData() ([]byte, string, *types.NamespacedName, error) {
// 	if m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
// 		return nil, "", nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
// 	}

// 	secret := &corev1.Secret{}
// 	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName}

// 	if err := m.Client.Get(context.TODO(), key, secret); err != nil {
// 		return nil, "", nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret %s for GCPMachinePool %s/%s", key.Name, m.Namespace(), m.Name())
// 	}

// 	value, ok := secret.Data["value"]
// 	if !ok {
// 		return nil, "", nil, errors.New("error retrieving bootstrap data: secret value key is missing")
// 	}

// 	return value, string(secret.Data["format"]), &key, nil
// }

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachinePoolScope) getBootstrapData(ctx context.Context) (string, error) {
	return GetBootstrapData(ctx, m.client, m.MachinePool, m.MachinePool.Spec.Template.Spec.Bootstrap)
}

// Replicas returns the desired number of replicas
func (m *MachinePoolScope) Replicas() *int32 {
	return m.MachinePool.Spec.Replicas
}

// Zones returns the targeted zones for the machine pool
func (m *MachinePoolScope) Zones() []string {
	zones := m.MachinePool.Spec.FailureDomains
	if len(zones) == 0 {
		failureDomains := m.ClusterGetter.FailureDomains()
		for zone := range failureDomains {
			zones = append(zones, zone)
		}
	}
	return zones
}

// Region returns the region for the GCP resources
func (m *MachinePoolScope) Region() string {
	return m.ClusterGetter.Region()
}

// // AdditionalTags merges AdditionalTags from the scope's GCPCluster and GCPMachinePool. If the same key is present in both,
// // the value from GCPMachinePool takes precedence. The returned Tags will never be nil.
// func (m *MachinePoolScope) AdditionalTags() infrav1.Tags {
// 	tags := make(infrav1.Tags)

// 	// Start with the cluster-wide tags...
// 	tags.Merge(m.InfraCluster.AdditionalTags())
// 	// ... and merge in the Machine's
// 	tags.Merge(m.GCPMachinePool.Spec.AdditionalTags)

// 	return tags
// }

// PatchObject persists the machinepool spec and status.
func (m *MachinePoolScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(
		ctx,
		m.GCPMachinePool,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			expinfrav1.MIGReadyCondition,
			expinfrav1.InstanceTemplateReadyCondition,
		}})
}

// PatchCAPIMachinePoolObject persists the capi machinepool configuration and status.
func (m *MachinePoolScope) PatchCAPIMachinePoolObject(ctx context.Context) error {
	return m.capiMachinePoolPatchHelper.Patch(
		ctx,
		m.MachinePool,
	)
}

// Close the MachinePoolScope by updating the machinepool spec, machine status.
func (m *MachinePoolScope) Close() error {
	return m.PatchObject(context.TODO())
}

// SetAnnotation sets a key value annotation on the GCPMachine.
func (m *MachinePoolScope) SetAnnotation(key, value string) {
	if m.GCPMachinePool.Annotations == nil {
		m.GCPMachinePool.Annotations = map[string]string{}
	}
	m.GCPMachinePool.Annotations[key] = value
}

// SetFailureMessage sets the GCPMachine status failure message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.GCPMachinePool.Status.FailureMessage = ptr.To[string](v.Error())
}

// SetFailureReason sets the GCPMachine status failure reason.
func (m *MachinePoolScope) SetFailureReason(v string) {
	m.GCPMachinePool.Status.FailureReason = &v
}

// HasFailed returns true when the GCPMachinePool's Failure reason or Failure message is populated.
func (m *MachinePoolScope) HasFailed() bool {
	return m.GCPMachinePool.Status.FailureReason != nil || m.GCPMachinePool.Status.FailureMessage != nil
}

// SetNotReady sets the GCPMachinePool Ready Status to false.
func (m *MachinePoolScope) SetNotReady() {
	m.GCPMachinePool.Status.Ready = false
}

// // GetMIGStatus returns the GCPMachinePool instance state from the status.
// func (m *MachinePoolScope) GetMIGStatus() *expinfrav1.MIGStatus {
// 	return m.GCPMachinePool.Status.MIGStatus
// }

// // SetMIGStatus sets the GCPMachinePool status instance state.
// func (m *MachinePoolScope) SetMIGStatus(v expinfrav1.MIGStatus) {
// 	m.GCPMachinePool.Status.MIGStatus = &v
// }

// GetObjectMeta returns the GCPMachinePool ObjectMeta.
func (m *MachinePoolScope) GetObjectMeta() *metav1.ObjectMeta {
	return &m.GCPMachinePool.ObjectMeta
}

// GetSetter returns the GCPMachinePool object setter.
func (m *MachinePoolScope) GetSetter() conditions.Setter {
	return m.GCPMachinePool
}

// // GetEC2Scope returns the EC2 scope.
// func (m *MachinePoolScope) GetEC2Scope() EC2Scope {
// 	return m.InfraCluster
// }

// // GetStatusInstanceTemplate returns the instanceTemplate from the status.
// func (m *MachinePoolScope) GetStatusInstanceTemplate() string {
// 	return m.GCPMachinePool.Status.InstanceTemplate
// }

// // SetStatusInstanceTemplate sets instanceTemplate in the status.
// func (m *MachinePoolScope) SetInstanceTemplate(id string) {
// 	m.GCPMachinePool.Status.InstanceTemplate = id
// }

// // GetInstanceTemplateLatestVersionStatus returns the launch template latest version status.
// func (m *MachinePoolScope) GetInstanceTemplateLatestVersionStatus() string {
// 	if m.GCPMachinePool.Status.InstanceTemplateVersion != nil {
// 		return *m.GCPMachinePool.Status.InstanceTemplateVersion
// 	}
// 	return ""
// }

// // SetInstanceTemplateLatestVersionStatus sets the launch template latest version status.
// func (m *MachinePoolScope) SetInstanceTemplateLatestVersionStatus(version string) {
// 	m.GCPMachinePool.Status.InstanceTemplateVersion = &version
// }

// // IsEKSManaged checks if the GCPMachinePool is EKS managed.
// func (m *MachinePoolScope) IsEKSManaged() bool {
// 	return m.InfraCluster.InfraCluster().GetObjectKind().GroupVersionKind().Kind == ekscontrolplanev1.GCPManagedControlPlaneKind
// }

// // SubnetIDs returns the machine pool subnet IDs.
// func (m *MachinePoolScope) SubnetIDs(subnetIDs []string) ([]string, error) {
// 	strategy, err := newDefaultSubnetPlacementStrategy(&m.Logger)
// 	if err != nil {
// 		return subnetIDs, fmt.Errorf("getting subnet placement strategy: %w", err)
// 	}

// 	return strategy.Place(&placementInput{
// 		SpecSubnetIDs:           subnetIDs,
// 		SpecAvailabilityZones:   m.GCPMachinePool.Spec.AvailabilityZones,
// 		ParentAvailabilityZones: m.MachinePool.Spec.FailureDomains,
// 		ControlplaneSubnets:     m.InfraCluster.Subnets(),
// 		SubnetPlacementType:     m.GCPMachinePool.Spec.AvailabilityZoneSubnetType,
// 	})
// }

// NodeStatus represents the status of a Kubernetes node.
type NodeStatus struct {
	Ready   bool
	Version string
}

// // UpdateInstanceStatuses ties ASG instances and Node status data together and updates GCPMachinePool
// // This updates if ASG instances ready and kubelet version running on the node..
// func (m *MachinePoolScope) UpdateInstanceStatuses(ctx context.Context, instances []infrav1.Instance) error {
// 	providerIDs := make([]string, len(instances))
// 	for i, instance := range instances {
// 		providerIDs[i] = fmt.Sprintf("gcp:////%s", instance.ID)
// 	}

// 	nodeStatusByProviderID, err := m.getNodeStatusByProviderID(ctx, providerIDs)
// 	if err != nil {
// 		return errors.Wrap(err, "failed to get node status by provider id")
// 	}

// 	var readyReplicas int32
// 	instanceStatuses := make([]expinfrav1.GCPMachinePoolInstanceStatus, len(instances))
// 	for i, instance := range instances {
// 		instanceStatuses[i] = expinfrav1.GCPMachinePoolInstanceStatus{
// 			InstanceID: instance.ID,
// 		}

// 		instanceStatus := instanceStatuses[i]
// 		if nodeStatus, ok := nodeStatusByProviderID[fmt.Sprintf("gcp:////%s", instanceStatus.InstanceID)]; ok {
// 			instanceStatus.Version = &nodeStatus.Version
// 			if nodeStatus.Ready {
// 				readyReplicas++
// 			}
// 		}
// 	}

// 	// TODO: readyReplicas can be used as status.replicas but this will delay machinepool to become ready. next reconcile updates this.
// 	m.GCPMachinePool.Status.Instances = instanceStatuses
// 	return nil
// }

// func (m *MachinePoolScope) getNodeStatusByProviderID(ctx context.Context, providerIDList []string) (map[string]*NodeStatus, error) {
// 	nodeStatusMap := map[string]*NodeStatus{}
// 	for _, id := range providerIDList {
// 		nodeStatusMap[id] = &NodeStatus{}
// 	}

// 	workloadClient, err := remote.NewClusterClient(ctx, "", m.Client, util.ObjectKey(m.Cluster))
// 	if err != nil {
// 		return nil, err
// 	}

// 	nodeList := corev1.NodeList{}
// 	for {
// 		if err := workloadClient.List(ctx, &nodeList, client.Continue(nodeList.Continue)); err != nil {
// 			return nil, errors.Wrapf(err, "failed to List nodes")
// 		}

// 		for _, node := range nodeList.Items {
// 			strList := strings.Split(node.Spec.ProviderID, "/")

// 			if status, ok := nodeStatusMap[fmt.Sprintf("gcp:////%s", strList[len(strList)-1])]; ok {
// 				status.Ready = nodeIsReady(node)
// 				status.Version = node.Status.NodeInfo.KubeletVersion
// 			}
// 		}

// 		if nodeList.Continue == "" {
// 			break
// 		}
// 	}

// 	return nodeStatusMap, nil
// }

// func nodeIsReady(node corev1.Node) bool {
// 	for _, n := range node.Status.Conditions {
// 		if n.Type == corev1.NodeReady {
// 			return n.Status == corev1.ConditionTrue
// 		}
// 	}
// 	return false
// }

// // GetInstanceTemplate returns the launch template.
// func (m *MachinePoolScope) GetInstanceTemplate() *expinfrav1.GCPInstanceTemplate {
// 	return &m.GCPMachinePool.Spec.GCPInstanceTemplate
// }

// // GetMachinePool returns the machine pool object.
// func (m *MachinePoolScope) GetMachinePool() *expclusterv1.MachinePool {
// 	return m.MachinePool
// }

// // InstanceTemplateName returns the name of the gcp instanceTemplate resource.
// func (m *MachinePoolScope) InstanceTemplateName() string {
// 	return m.Name()
// }

// InstanceGroupManagerResourceName is the name to use for the instanceGroupManager GCP resource
func (m *MachinePoolScope) InstanceGroupManagerResourceName() (*meta.Key, error) {
	name := m.Name() // TODO: Sanitization?

	zones := m.Zones()
	if len(zones) != 1 {
		// TODO: Support regional instanceGroupManagers
		return nil, fmt.Errorf("instanceGroupManager must be created in a single zone, got %d zones (%s)", len(zones), strings.Join(zones, ","))
	}
	zone := zones[0]
	igmKey := meta.ZonalKey(name, zone)

	return igmKey, nil
}

// InstanceGroupManagerResource is the desired state for the instanceGroupManager GCP resource
func (m *MachinePoolScope) InstanceGroupManagerResource(instanceTemplate *meta.Key) (*compute.InstanceGroupManager, error) {
	instanceTemplateSelfLink := gcp.SelfLink("instanceTemplates", instanceTemplate)
	baseInstanceName := limitStringLength(m.Name(), 58) // TODO: Sanitization

	zones := m.Zones()
	if len(zones) == 0 {
		return nil, fmt.Errorf("must specify at least one zone")
	}

	replicas := int64(1)
	if p := m.MachinePool.Spec.Replicas; p != nil {
		replicas = int64(*p)
	}
	// instanceSpec := s.scope.InstanceSpec(log)
	// instanceName := instanceSpec.Name

	// namePrefix := instanceSpec.Name[:50] + "-"

	desired := &compute.InstanceGroupManager{
		BaseInstanceName: baseInstanceName,
		Description:      "", // TODO
		InstanceTemplate: instanceTemplateSelfLink,
		// ListManagedInstancesResults: "PAGINATED", // TODO
		TargetSize: replicas,
		// TargetStoppedSize:   targetStoppedSize,
		// TargetSuspendedSize: targetSuspendedSize,
	}

	if len(zones) > 1 {
		desired.DistributionPolicy = &compute.DistributionPolicy{}
		for _, zone := range zones {
			zoneSelfLink, err := buildZoneSelfLink(zone)
			if err != nil {
				return nil, err
			}
			desired.DistributionPolicy.Zones = append(desired.DistributionPolicy.Zones, &compute.DistributionPolicyZoneConfiguration{
				Zone: zoneSelfLink,
			})
		}
	} else {
		zoneSelfLink, err := buildZoneSelfLink(zones[0])
		if err != nil {
			return nil, err
		}
		desired.Zone = zoneSelfLink
	}

	return desired, nil
}

// buildZoneSelfLink returns a fully-qualified zone link from a user-provided zone
func buildZoneSelfLink(zone string) (string, error) {
	tokens := strings.Split(zone, "/")
	if len(tokens) == 1 {
		return "zones/" + tokens[0], nil
	}
	return "", fmt.Errorf("zone %q was not a recognized format", zone)
}

// BaseInstanceTemplateResourceName is the base name to use for the instanceTemplate GCP resource.
// The instance template is immutable, so we add a suffix that hash-encodes the version
func (m *MachinePoolScope) BaseInstanceTemplateResourceName() (*meta.Key, error) {
	name := m.Name() // TODO: Sanitization?

	// We only use the first 46 characters, to leave room for a 16 character hash
	// 63 characters max, 16 character hash; 1 hyphen
	namePrefix := limitStringLength(name, 63-16-1) + "-"

	region := m.Region()
	return meta.RegionalKey(namePrefix, region), nil
}

// limitStringLength returns the string truncated to the specified maximum length.
func limitStringLength(s string, maxLength int) string {
	if len(s) > maxLength {
		return s[:maxLength]
	}
	return s
}

// InstanceTemplateResource is the desired state for the instanceTemplate GCP resource
func (m *MachinePoolScope) InstanceTemplateResource(ctx context.Context) (*compute.InstanceTemplate, error) {
	log := logger.FromContext(ctx)

	bootstrapData, err := m.getBootstrapData(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving bootstrap data for instanceTemplate: %w", err)
	}

	instance := &compute.InstanceProperties{
		// Name:        m.Name(),
		// 		Zone:        m.Zone(),
		MachineType: m.GCPMachinePool.Spec.InstanceType,
		Tags: &compute.Tags{
			Items: append(
				m.GCPMachinePool.Spec.AdditionalNetworkTags,
				fmt.Sprintf("%s-%s", m.ClusterGetter.Name(), m.Role()),
				m.ClusterGetter.Name(),
			),
		},
		ResourceManagerTags: shared.ResourceTagConvert(ctx, m.ResourceManagerTags()),
		Labels: infrav1.Build(infrav1.BuildParams{
			ClusterName: m.ClusterGetter.Name(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Role:        ptr.To[string](m.Role()),
			//nolint: godox
			// TODO: Check what needs to be added for the cloud provider label.
			Additional: m.ClusterGetter.AdditionalLabels().AddLabels(m.GCPMachinePool.Spec.AdditionalLabels),
		}),
		Scheduling: &compute.Scheduling{
			Preemptible: m.GCPMachinePool.Spec.Preemptible,
		},
	}

	if m.GCPMachinePool.Spec.ProvisioningModel != nil {
		// TODO: Can we dedup with MachinePool logic - until then we have to keep them in sync manually

		switch *m.GCPMachinePool.Spec.ProvisioningModel {
		case infrav1.ProvisioningModelSpot:
			instance.Scheduling.ProvisioningModel = "SPOT"
		case infrav1.ProvisioningModelStandard:
			instance.Scheduling.ProvisioningModel = "STANDARD"
		default:
			return nil, fmt.Errorf("unknown ProvisioningModel value: %q", *m.GCPMachinePool.Spec.ProvisioningModel)
		}
	}

	instance.CanIpForward = true
	if m.GCPMachinePool.Spec.IPForwarding != nil && *m.GCPMachinePool.Spec.IPForwarding == infrav1.IPForwardingDisabled {
		// TODO: Can we dedup with MachinePool logic - until then we have to keep them in sync manually

		instance.CanIpForward = false
	}
	if config := m.GCPMachinePool.Spec.ShieldedInstanceConfig; config != nil {
		// TODO: Can we dedup with MachinePool logic - until then we have to keep them in sync manually

		instance.ShieldedInstanceConfig = &compute.ShieldedInstanceConfig{
			EnableSecureBoot:          false,
			EnableVtpm:                true,
			EnableIntegrityMonitoring: true,
		}
		if config.SecureBoot == infrav1.SecureBootPolicyEnabled {
			instance.ShieldedInstanceConfig.EnableSecureBoot = true
		}
		if config.VirtualizedTrustedPlatformModule == infrav1.VirtualizedTrustedPlatformModulePolicyDisabled {
			instance.ShieldedInstanceConfig.EnableVtpm = false
		}
		if config.IntegrityMonitoring == infrav1.IntegrityMonitoringPolicyDisabled {
			instance.ShieldedInstanceConfig.EnableIntegrityMonitoring = false
		}
	}
	if onHostMaintenance := ValueOf(m.GCPMachinePool.Spec.OnHostMaintenance); onHostMaintenance != "" {
		// TODO: Can we dedup with MachinePool logic - until then we have to keep them in sync manually

		switch onHostMaintenance {
		case infrav1.HostMaintenancePolicyMigrate:
			instance.Scheduling.OnHostMaintenance = "MIGRATE"
		case infrav1.HostMaintenancePolicyTerminate:
			instance.Scheduling.OnHostMaintenance = "TERMINATE"
		default:
			log.Error(errors.New("Invalid value"), "Unknown OnHostMaintenance value", "Spec.OnHostMaintenance", onHostMaintenance)
			instance.Scheduling.OnHostMaintenance = strings.ToUpper(string(onHostMaintenance))
		}
	}

	if confidentialCompute := m.GCPMachinePool.Spec.ConfidentialCompute; confidentialCompute != nil {
		// TODO: Can we dedup with MachinePool logic - until then we have to keep them in sync manually
		enabled := *confidentialCompute != infrav1.ConfidentialComputePolicyDisabled
		instance.ConfidentialInstanceConfig = &compute.ConfidentialInstanceConfig{
			EnableConfidentialCompute: enabled,
		}
		switch *confidentialCompute {
		case infrav1.ConfidentialComputePolicySEV:
			instance.ConfidentialInstanceConfig.ConfidentialInstanceType = "SEV"
		case infrav1.ConfidentialComputePolicySEVSNP:
			instance.ConfidentialInstanceConfig.ConfidentialInstanceType = "SEV_SNP"
		case infrav1.ConfidentialComputePolicyTDX:
			instance.ConfidentialInstanceConfig.ConfidentialInstanceType = "TDX"
		default:
		}
	}

	instance.Disks = append(instance.Disks, m.InstanceImageSpec(ctx))
	instance.Disks = append(instance.Disks, m.InstanceAdditionalDiskSpec()...)
	instance.Metadata = InstanceAdditionalMetadataSpec(m.GCPMachinePool.Spec.AdditionalMetadata)
	instance.ServiceAccounts = append(instance.ServiceAccounts, InstanceServiceAccountsSpec(m.GCPMachinePool.Spec.ServiceAccount))
	instance.NetworkInterfaces = append(instance.NetworkInterfaces, InstanceNetworkInterfaceSpec(m.ClusterGetter, m.GCPMachinePool.Spec.PublicIP, m.GCPMachinePool.Spec.Subnet))

	instance.Metadata.Items = append(instance.Metadata.Items, &compute.MetadataItems{
		Key:   "user-data",
		Value: ptr.To[string](bootstrapData),
	})

	instanceTemplate := &compute.InstanceTemplate{
		Region:     m.Region(),
		Properties: instance,
	}

	// // Name: Name of the resource; provided by the client when the resource is
	// // created. The name must be 1-63 characters long, and comply with RFC1035.
	// // Specifically, the name must be 1-63 characters long and match the regular
	// // expression `[a-z]([-a-z0-9]*[a-z0-9])?` which means the first character must
	// // be a lowercase letter, and all following characters must be a dash,
	// // lowercase letter, or digit, except the last character, which cannot be a
	// // dash.
	// Name string `json:"name,omitempty"`

	return instanceTemplate, nil
}

// InstanceImageSpec returns compute instance image attched-disk spec.
func (m *MachinePoolScope) InstanceImageSpec(ctx context.Context) *compute.AttachedDisk {
	// TODO: Can we dedup with MachinePool InstanceImageSpec - until then we have to keep them in sync manually
	spec := m.GCPMachinePool.Spec

	version := ""
	if m.MachinePool.Spec.Template.Spec.Version != nil {
		version = *m.MachinePool.Spec.Template.Spec.Version
	}

	image := "capi-ubuntu-1804-k8s-" + strings.ReplaceAll(semver.MajorMinor(version), ".", "-")
	sourceImage := path.Join("projects", m.ClusterGetter.Project(), "global", "images", "family", image)
	if spec.Image != nil {
		sourceImage = *spec.Image
	} else if spec.ImageFamily != nil {
		sourceImage = *spec.ImageFamily
	}

	diskType := infrav1.PdStandardDiskType
	if t := spec.RootDeviceType; t != nil {
		diskType = *t
	}

	// TODO: diskType = path.Join("zones", m.Zone(), "diskTypes", string(diskType)),

	disk := &compute.AttachedDisk{
		AutoDelete: true,
		Boot:       true,
		InitializeParams: &compute.AttachedDiskInitializeParams{
			DiskSizeGb:          spec.RootDeviceSize,
			DiskType:            string(diskType),
			ResourceManagerTags: shared.ResourceTagConvert(ctx, spec.ResourceManagerTags),
			SourceImage:         sourceImage,
			Labels:              m.ClusterGetter.AdditionalLabels().AddLabels(spec.AdditionalLabels),
		},
	}

	if spec.RootDiskEncryptionKey != nil {
		if spec.RootDiskEncryptionKey.KeyType == infrav1.CustomerManagedKey && spec.RootDiskEncryptionKey.ManagedKey != nil {
			disk.DiskEncryptionKey = &compute.CustomerEncryptionKey{
				KmsKeyName: spec.RootDiskEncryptionKey.ManagedKey.KMSKeyName,
			}
			if spec.RootDiskEncryptionKey.KMSKeyServiceAccount != nil {
				disk.DiskEncryptionKey.KmsKeyServiceAccount = *spec.RootDiskEncryptionKey.KMSKeyServiceAccount
			}
		} else if spec.RootDiskEncryptionKey.KeyType == infrav1.CustomerSuppliedKey && spec.RootDiskEncryptionKey.SuppliedKey != nil {
			disk.DiskEncryptionKey = &compute.CustomerEncryptionKey{
				RawKey:          string(spec.RootDiskEncryptionKey.SuppliedKey.RawKey),
				RsaEncryptedKey: string(spec.RootDiskEncryptionKey.SuppliedKey.RSAEncryptedKey),
			}
			if spec.RootDiskEncryptionKey.KMSKeyServiceAccount != nil {
				disk.DiskEncryptionKey.KmsKeyServiceAccount = *spec.RootDiskEncryptionKey.KMSKeyServiceAccount
			}
		}
	}

	return disk
}

// InstanceAdditionalDiskSpec returns compute instance additional attched-disk spec.
func (m *MachinePoolScope) InstanceAdditionalDiskSpec() []*compute.AttachedDisk {
	// TODO: Can we dedup with MachinePool InstanceImageSpec - until then we have to keep them in sync manually

	spec := m.GCPMachinePool.Spec

	additionalDisks := make([]*compute.AttachedDisk, 0, len(spec.AdditionalDisks))
	for _, disk := range spec.AdditionalDisks {
		diskType := string(ValueOf(disk.DeviceType))

		// TODO: // path.Join("zones", m.Zone(), "diskTypes", string(*disk.DeviceType))

		additionalDisk := &compute.AttachedDisk{
			AutoDelete: true,
			InitializeParams: &compute.AttachedDiskInitializeParams{
				DiskSizeGb:          ptr.Deref(disk.Size, 30),
				DiskType:            diskType,
				ResourceManagerTags: shared.ResourceTagConvert(context.TODO(), spec.ResourceManagerTags),
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
		if disk.EncryptionKey != nil {
			if spec.RootDiskEncryptionKey.KeyType == infrav1.CustomerManagedKey && spec.RootDiskEncryptionKey.ManagedKey != nil {
				additionalDisk.DiskEncryptionKey = &compute.CustomerEncryptionKey{
					KmsKeyName: spec.RootDiskEncryptionKey.ManagedKey.KMSKeyName,
				}
				if spec.RootDiskEncryptionKey.KMSKeyServiceAccount != nil {
					additionalDisk.DiskEncryptionKey.KmsKeyServiceAccount = *spec.RootDiskEncryptionKey.KMSKeyServiceAccount
				}
			} else if spec.RootDiskEncryptionKey.KeyType == infrav1.CustomerSuppliedKey && spec.RootDiskEncryptionKey.SuppliedKey != nil {
				additionalDisk.DiskEncryptionKey = &compute.CustomerEncryptionKey{
					RawKey:          string(spec.RootDiskEncryptionKey.SuppliedKey.RawKey),
					RsaEncryptedKey: string(spec.RootDiskEncryptionKey.SuppliedKey.RSAEncryptedKey),
				}
				if spec.RootDiskEncryptionKey.KMSKeyServiceAccount != nil {
					additionalDisk.DiskEncryptionKey.KmsKeyServiceAccount = *spec.RootDiskEncryptionKey.KMSKeyServiceAccount
				}
			}
		}

		additionalDisks = append(additionalDisks, additionalDisk)
	}

	return additionalDisks
}

// // GetRuntimeObject returns the GCPMachinePool object, in runtime.Object form.
// func (m *MachinePoolScope) GetRuntimeObject() runtime.Object {
// 	return m.GCPMachinePool
// }

// ResourceManagerTags merges ResourceManagerTags from the scope's GCPCluster and GCPMachine. If the same key is present in both,
// the value from GCPMachine takes precedence. The returned ResourceManagerTags will never be nil.
func (m *MachinePoolScope) ResourceManagerTags() infrav1.ResourceManagerTags {
	tags := infrav1.ResourceManagerTags{}

	// Start with the cluster-wide tags...
	tags.Merge(m.ClusterGetter.ResourceManagerTags())
	// ... and merge in the Machine's
	tags.Merge(m.GCPMachinePool.Spec.ResourceManagerTags)

	return tags
}

// Role returns the machine role from the labels.
func (m *MachinePoolScope) Role() string {
	// TODO: Or template labels?
	_, isControlPlane := m.MachinePool.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel]

	if isControlPlane {
		// TODO: Extract constants
		return "control-plane"
	}

	return "node"
}

// // GetLifecycleHooks returns the desired lifecycle hooks for the ASG.
// func (m *MachinePoolScope) GetLifecycleHooks() []expinfrav1.GCPLifecycleHook {
// 	return m.GCPMachinePool.Spec.GCPLifecycleHooks
// }

func ValueOf[V any](v *V) V {
	if v != nil {
		return *v
	}
	var zero V
	return zero
}
