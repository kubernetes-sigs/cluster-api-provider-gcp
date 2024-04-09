/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// MachinePoolNameLabel indicates the GCPMachinePool name the GCPMachinePoolMachine belongs.
	MachinePoolNameLabel = "gcpmachinepool.infrastructure.cluster.x-k8s.io/machine-pool"

	// RollingUpdateGCPMachinePoolDeploymentStrategyType replaces GCPMachinePoolMachines with older models with
	// GCPMachinePoolMachines based on the latest model.
	// i.e. gradually scale down the old GCPMachinePoolMachines and scale up the new ones.
	RollingUpdateGCPMachinePoolDeploymentStrategyType GCPMachinePoolDeploymentStrategyType = "RollingUpdate"

	// OldestDeletePolicyType will delete machines with the oldest creation date first.
	OldestDeletePolicyType GCPMachinePoolDeletePolicyType = "Oldest"
	// NewestDeletePolicyType will delete machines with the newest creation date first.
	NewestDeletePolicyType GCPMachinePoolDeletePolicyType = "Newest"
	// RandomDeletePolicyType will delete machines in random order.
	RandomDeletePolicyType GCPMachinePoolDeletePolicyType = "Random"
)

const (
	// PdStandardDiskType defines the name for the standard disk.
	PdStandardDiskType DiskType = "pd-standard"
	// PdSsdDiskType defines the name for the ssd disk.
	PdSsdDiskType DiskType = "pd-ssd"
	// LocalSsdDiskType defines the name for the local ssd disk.
	LocalSsdDiskType DiskType = "local-ssd"
)

// AttachedDiskSpec degined GCP machine disk.
type AttachedDiskSpec struct {
	// DeviceType is a device type of the attached disk.
	// Supported types of non-root attached volumes:
	// 1. "pd-standard" - Standard (HDD) persistent disk
	// 2. "pd-ssd" - SSD persistent disk
	// 3. "local-ssd" - Local SSD disk (https://cloud.google.com/compute/docs/disks/local-ssd).
	// Default is "pd-standard".
	// +kubebuilder:validation:Enum=pd-standard;pd-ssd;local-ssd
	// +optional
	DeviceType *string `json:"deviceType,omitempty"`
	// Size is the size of the disk in GBs.
	// Defaults to 30GB. For "local-ssd" size is always 375GB.
	// +optional
	Size *int64 `json:"size,omitempty"`
}

// ServiceAccount describes compute.serviceAccount.
type ServiceAccount struct {
	// Email: Email address of the service account.
	Email string `json:"email,omitempty"`

	// Scopes: The list of scopes to be made available for this service
	// account.
	Scopes []string `json:"scopes,omitempty"`
}

// MetadataItem is a key/value pair to add to the instance's metadata.
type MetadataItem struct {
	// Key is the identifier for the metadata entry.
	Key string `json:"key"`
	// Value is the value of the metadata entry.
	Value *string `json:"value,omitempty"`
}

// GCPMachinePoolSpec defines the desired state of GCPMachinePool and the GCP instances that it will create.
type GCPMachinePoolSpec struct {

	// AdditionalDisks are optional non-boot attached disks.
	// +optional
	AdditionalDisks []AttachedDiskSpec `json:"additionalDisks,omitempty"`

	// AdditionalNetworkTags is a list of network tags that should be applied to the
	// instance. These tags are set in addition to any network tags defined
	// at the cluster level or in the actuator.
	// +optional
	AdditionalNetworkTags []string `json:"additionalNetworkTags,omitempty"`

	// AdditionalMetadata is an optional set of metadata to add to an instance, in addition to the ones added by default by the
	// GCP provider.
	// +listType=map
	// +listMapKey=key
	// +optional
	AdditionalMetadata []MetadataItem `json:"additionalMetadata,omitempty"`

	// AdditionalLabels is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// GCP provider. If both the GCPCluster and the GCPMachinePool specify the same tag name with different values, the
	// GCPMachinePool's value takes precedence.
	// +optional
	AdditionalLabels infrav1.Labels `json:"additionalLabels,omitempty"`

	// InstanceType is the type of instance to create. Example: n1.standard-2
	InstanceType string `json:"instanceType"`

	// Image is the full reference to a valid image to be used for this machine.
	// Takes precedence over ImageFamily.
	// +optional
	Image *string `json:"image,omitempty"`

	// ImageFamily is the family of the image to be used for this machine.
	// +optional
	ImageFamily *string `json:"imageFamily,omitempty"`

	// Location is the GCP region location ex us-central1
	Location string `json:"location"`

	// MinCPUPlatform is the minimum CPU platform to use for the instance. The instance may be scheduled on the specified or newer CPU platform.
	// applicable values are the friendly names of CPU platforms, such as minCpuPlatform: "Intel Haswell" or minCpuPlatform: "Intel Sandy Bridge".
	// +optional
	MinCPUPlatform *string `json:"minCPUPlatform,omitempty"`

	// Network is the network to be used by machines in the machine pool.
	Network string `json:"network"`

	// PublicIP specifies whether the instance should get a public IP.
	// Set this to true if you don't have a NAT instances or Cloud Nat setup.
	// +optional
	PublicIP *bool `json:"publicIP,omitempty"`

	// ProviderID is the identification ID of the Managed Instance Group
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// ProviderIDList is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`

	// RootDeviceSize is the size of the root volume in GB.
	// Defaults to 30.
	// +optional
	RootDeviceSize int64 `json:"rootDeviceSize,omitempty"`

	// RootDeviceType is the type of the root volume.
	// Supported types of root volumes:
	// 1. "pd-standard" - Standard (HDD) persistent disk
	// 2. "pd-ssd" - SSD persistent disk
	// Default is "pd-standard".
	// +optional
	RootDeviceType *DiskType `json:"rootDeviceType,omitempty"`

	// ServiceAccount specifies the service account email and which scopes to assign to the machine.
	// Defaults to: email: "default", scope: []{compute.CloudPlatformScope}
	// +optional
	ServiceAccount *ServiceAccount `json:"serviceAccounts,omitempty"`

	// Subnet is a reference to the subnetwork to use for this instance. If not specified,
	// the first subnetwork retrieved from the Cluster Region and Network is picked.
	// +optional
	Subnet *string `json:"subnet,omitempty"`

	// The deployment strategy to use to replace existing GCPMachinePoolMachines with new ones.
	// +optional
	// +kubebuilder:default={type: "RollingUpdate", rollingUpdate: {maxSurge: 1, maxUnavailable: 0, deletePolicy: Oldest}}
	Strategy GCPMachinePoolDeploymentStrategy `json:"strategy,omitempty"`

	// NodeDrainTimeout is the total amount of time that the controller will spend on draining a node.
	// The default value is 0, meaning that the node can be drained without any time limitations.
	// NOTE: NodeDrainTimeout is different from `kubectl drain --timeout`
	// +optional
	NodeDrainTimeout *metav1.Duration `json:"nodeDrainTimeout,omitempty"`

	// Zone is the GCP zone location ex us-central1-a
	Zone string `json:"zone"`
}

// GCPMachinePoolDeploymentStrategyType is the type of deployment strategy employed to rollout a new version of the GCPMachinePool.
type GCPMachinePoolDeploymentStrategyType string

// GCPMachinePoolDeploymentStrategy describes how to replace existing machines with new ones.
type GCPMachinePoolDeploymentStrategy struct {
	// Type of deployment. Currently the only supported strategy is RollingUpdate
	// +optional
	// +kubebuilder:validation:Enum=RollingUpdate
	// +optional
	// +kubebuilder:default=RollingUpdate
	Type GCPMachinePoolDeploymentStrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if
	// MachineDeploymentStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *MachineRollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

// GCPMachinePoolDeletePolicyType is the type of DeletePolicy employed to select machines to be deleted during an
// upgrade.
type GCPMachinePoolDeletePolicyType string

// MachineRollingUpdateDeployment is used to control the desired behavior of rolling update.
type MachineRollingUpdateDeployment struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	// +kubebuilder:default:=0
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	// +kubebuilder:default:=1
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// DeletePolicy defines the policy used by the MachineDeployment to identify nodes to delete when downscaling.
	// Valid values are "Random, "Newest", "Oldest"
	// When no value is supplied, the default is Oldest
	// +optional
	// +kubebuilder:validation:Enum=Random;Newest;Oldest
	// +kubebuilder:default:=Oldest
	DeletePolicy GCPMachinePoolDeletePolicyType `json:"deletePolicy,omitempty"`
}

// GCPMachinePoolStatus defines the observed state of GCPMachinePool and the GCP instances that it manages.
type GCPMachinePoolStatus struct {

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// The number of non-terminated machines targeted by this machine pool that have the desired template spec.
	// +optional
	Replicas int32 `json:"replicas"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the MachinePool and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the MachinePool's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of MachinePools
	// can be added as events to the MachinePool object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the MachinePool and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the MachinePool's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of MachinePools
	// can be added as events to the MachinePool object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions specifies the conditions for the managed machine pool
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this GCPMachine belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"

// GCPMachinePool is the Schema for the gcpmachinepools API and represents a GCP Machine Pool.
type GCPMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPMachinePoolSpec   `json:"spec,omitempty"`
	Status GCPMachinePoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GCPMachinePoolList contains a list of GCPMachinePool resources.
type GCPMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPMachinePool `json:"items"`
}

// GetConditions returns the conditions for the GCPManagedMachinePool.
func (r *GCPMachinePool) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the status conditions for the GCPManagedMachinePool.
func (r *GCPMachinePool) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
func init() {
	infrav1.SchemeBuilder.Register(&GCPMachinePool{}, &GCPMachinePoolList{})
}
