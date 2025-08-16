/*
Copyright 2021 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/runtime/schema"

	capg "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Constants block.
const (
	// LaunchTemplateLatestVersion defines the launching of the latest version of the template.
	LaunchTemplateLatestVersion = "$Latest"
)

// GCPMachinePoolSpec defines the desired state of GCPMachinePool.
type GCPMachinePoolSpec struct {
	// // ProviderID is the ARN of the associated ASG
	// // +optional
	// ProviderID string `json:"providerID,omitempty"`

	// // MinSize defines the minimum size of the group.
	// // +kubebuilder:default=1
	// // +kubebuilder:validation:Minimum=0
	// MinSize int32 `json:"minSize"`

	// // MaxSize defines the maximum size of the group.
	// // +kubebuilder:default=1
	// // +kubebuilder:validation:Minimum=1
	// MaxSize int32 `json:"maxSize"`

	// InstanceType is the type of instance to create. Example: n1.standard-2
	InstanceType string `json:"instanceType"`

	// Subnet is a reference to the subnetwork to use for this instance. If not specified,
	// the first subnetwork retrieved from the Cluster Region and Network is picked.
	// +optional
	Subnet *string `json:"subnet,omitempty"`

	// // ProviderID is the unique identifier as specified by the cloud provider.
	// // +optional
	// ProviderID *string `json:"providerID,omitempty"`

	// ImageFamily is the full reference to a valid image family to be used for this machine.
	// +optional
	ImageFamily *string `json:"imageFamily,omitempty"`

	// Image is the full reference to a valid image to be used for this machine.
	// Takes precedence over ImageFamily.
	// +optional
	Image *string `json:"image,omitempty"`

	// AdditionalLabels is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// GCP provider. If both the GCPCluster and the GCPMachinePool specify the same tag name with different values, the
	// GCPMachinePool's value takes precedence.
	// +optional
	AdditionalLabels capg.Labels `json:"additionalLabels,omitempty"`

	// AdditionalMetadata is an optional set of metadata to add to an instance, in addition to the ones added by default by the
	// GCP provider.
	// +listType=map
	// +listMapKey=key
	// +optional
	AdditionalMetadata []capg.MetadataItem `json:"additionalMetadata,omitempty"`

	// // IAMInstanceProfile is a name of an IAM instance profile to assign to the instance
	// // +optional
	// // IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// PublicIP specifies whether the instance should get a public IP.
	// Set this to true if you don't have a NAT instances or Cloud Nat setup.
	// +optional
	PublicIP *bool `json:"publicIP,omitempty"`

	// AdditionalNetworkTags is a list of network tags that should be applied to the
	// instance. These tags are set in addition to any network tags defined
	// at the cluster level or in the actuator.
	// +optional
	AdditionalNetworkTags []string `json:"additionalNetworkTags,omitempty"`

	// ResourceManagerTags is an optional set of tags to apply to GCP resources managed
	// by the GCP provider. GCP supports a maximum of 50 tags per resource.
	// +maxItems=50
	// +optional
	ResourceManagerTags capg.ResourceManagerTags `json:"resourceManagerTags,omitempty"`

	// RootDeviceSize is the size of the root volume in GB.
	// Defaults to 30.
	// +optional
	RootDeviceSize int64 `json:"rootDeviceSize,omitempty"`

	// RootDeviceType is the type of the root volume.
	// Supported types of root volumes:
	// 1. "pd-standard" - Standard (HDD) persistent disk
	// 2. "pd-ssd" - SSD persistent disk
	// 3. "pd-balanced" - Balanced Persistent Disk
	// 4. "hyperdisk-balanced" - Hyperdisk Balanced
	// Default is "pd-standard".
	// +optional
	RootDeviceType *capg.DiskType `json:"rootDeviceType,omitempty"`

	// AdditionalDisks are optional non-boot attached disks.
	// +optional
	AdditionalDisks []capg.AttachedDiskSpec `json:"additionalDisks,omitempty"`

	// ServiceAccount specifies the service account email and which scopes to assign to the machine.
	// Defaults to: email: "default", scope: []{compute.CloudPlatformScope}
	// +optional
	ServiceAccount *capg.ServiceAccount `json:"serviceAccounts,omitempty"`

	// Preemptible defines if instance is preemptible
	// +optional
	Preemptible bool `json:"preemptible,omitempty"`

	// ProvisioningModel defines if instance is spot.
	// If set to "Standard" while preemptible is true, then the VM will be of type "Preemptible".
	// If "Spot", VM type is "Spot". When unspecified, defaults to "Standard".
	// +kubebuilder:validation:Enum=Standard;Spot
	// +optional
	ProvisioningModel *capg.ProvisioningModel `json:"provisioningModel,omitempty"`

	// IPForwarding Allows this instance to send and receive packets with non-matching destination or source IPs.
	// This is required if you plan to use this instance to forward routes. Defaults to enabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	// +kubebuilder:default=Enabled
	// +optional
	IPForwarding *capg.IPForwarding `json:"ipForwarding,omitempty"`

	// ShieldedInstanceConfig is the Shielded VM configuration for this machine
	// +optional
	ShieldedInstanceConfig *capg.GCPShieldedInstanceConfig `json:"shieldedInstanceConfig,omitempty"`

	// OnHostMaintenance determines the behavior when a maintenance event occurs that might cause the instance to reboot.
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is "Migrate".
	// +kubebuilder:validation:Enum=Migrate;Terminate;
	// +optional
	OnHostMaintenance *capg.HostMaintenancePolicy `json:"onHostMaintenance,omitempty"`

	// ConfidentialCompute Defines whether the instance should have confidential compute enabled or not, and the confidential computing technology of choice.
	// If Disabled, the machine will not be configured to be a confidential computing instance.
	// If Enabled, confidential computing will be configured and AMD Secure Encrypted Virtualization will be configured by default. That is subject to change over time. If using AMD Secure Encrypted Virtualization is vital, use AMDEncryptedVirtualization explicitly instead.
	// If AMDEncryptedVirtualization, it will configure AMD Secure Encrypted Virtualization (AMD SEV) as the confidential computing technology.
	// If AMDEncryptedVirtualizationNestedPaging, it will configure AMD Secure Encrypted Virtualization Secure Nested Paging (AMD SEV-SNP) as the confidential computing technology.
	// If IntelTrustedDomainExtensions, it will configure Intel TDX as the confidential computing technology.
	// If enabled (any value other than Disabled) OnHostMaintenance is required to be set to "Terminate".
	// If omitted, the platform chooses a default, which is subject to change over time, currently that default is false.
	// +kubebuilder:validation:Enum=Enabled;Disabled;AMDEncryptedVirtualization;AMDEncryptedVirtualizationNestedPaging;IntelTrustedDomainExtensions
	// +optional
	ConfidentialCompute *capg.ConfidentialComputePolicy `json:"confidentialCompute,omitempty"`

	// RootDiskEncryptionKey defines the KMS key to be used to encrypt the root disk.
	// +optional
	RootDiskEncryptionKey *capg.CustomerEncryptionKey `json:"rootDiskEncryptionKey,omitempty"`

	// // AvailabilityZones is an array of availability zones instances can run in
	// AvailabilityZones []string `json:"availabilityZones,omitempty"`

	// // Subnets is an array of subnet configurations
	// // +optional
	// Subnets []infrav1.AWSResourceReference `json:"subnets,omitempty"`

	// // AdditionalTags is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// // AWS provider.
	// // +optional
	// AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

	// // AWSLaunchTemplate specifies the launch template and version to use when an instance is launched.
	// // +kubebuilder:validation:Required
	// AWSLaunchTemplate AWSLaunchTemplate `json:"awsLaunchTemplate"`

	// // MixedInstancesPolicy describes how multiple instance types will be used by the ASG.
	// MixedInstancesPolicy *MixedInstancesPolicy `json:"mixedInstancesPolicy,omitempty"`

	// // ProviderIDList are the identification IDs of machine instances provided by the provider.
	// // This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// // +optional
	// ProviderIDList []string `json:"providerIDList,omitempty"`

	// // The amount of time, in seconds, after a scaling activity completes before another scaling activity can start.
	// // If no value is supplied by user a default value of 300 seconds is set
	// // +optional
	// DefaultCoolDown metav1.Duration `json:"defaultCoolDown,omitempty"`

	// // RefreshPreferences describes set of preferences associated with the instance refresh request.
	// // +optional
	// RefreshPreferences *RefreshPreferences `json:"refreshPreferences,omitempty"`

	// // Enable or disable the capacity rebalance autoscaling group feature
	// // +optional
	// CapacityRebalance bool `json:"capacityRebalance,omitempty"`
}

// // RefreshPreferences defines the specs for instance refreshing.
// type RefreshPreferences struct {
// 	// The strategy to use for the instance refresh. The only valid value is Rolling.
// 	// A rolling update is an update that is applied to all instances in an Auto
// 	// Scaling group until all instances have been updated.
// 	// +optional
// 	Strategy *string `json:"strategy,omitempty"`

// 	// The number of seconds until a newly launched instance is configured and ready
// 	// to use. During this time, the next replacement will not be initiated.
// 	// The default is to use the value for the health check grace period defined for the group.
// 	// +optional
// 	InstanceWarmup *int64 `json:"instanceWarmup,omitempty"`

// 	// The amount of capacity as a percentage in ASG that must remain healthy
// 	// during an instance refresh. The default is 90.
// 	// +optional
// 	MinHealthyPercentage *int64 `json:"minHealthyPercentage,omitempty"`
// }

// GCPMachinePoolStatus defines the observed state of GCPMachinePool.
type GCPMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	// Conditions defines current service state of the GCPMachinePool.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// // Instances contains the status for each instance in the pool
	// // +optional
	// Instances []GCPMachinePoolInstanceStatus `json:"instances,omitempty"`

	// // The ID of the instance template
	// InstanceTemplate string `json:"instanceTemplate,omitempty"`

	// // The version of the launch template
	// // +optional
	// LaunchTemplateVersion *string `json:"launchTemplateVersion,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// MIGStatus *MIGStatus `json:"migStatus,omitempty"`
}

// GCPMachinePoolInstanceStatus defines the status of the GCPMachinePoolInstance.
type GCPMachinePoolInstanceStatus struct {
	// // InstanceID is the identification of the Machine Instance within ASG
	// // +optional
	// InstanceID string `json:"instanceID,omitempty"`

	// // Version defines the Kubernetes version for the Machine Instance
	// // +optional
	// Version *string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=gcpmachinepools,scope=Namespaced,categories=cluster-api,shortName=gcpmp
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Machine ready status"
// +kubebuilder:printcolumn:name="MinSize",type="integer",JSONPath=".spec.minSize",description="Minimum instanes in ASG"
// +kubebuilder:printcolumn:name="MaxSize",type="integer",JSONPath=".spec.maxSize",description="Maximum instanes in ASG"
// +kubebuilder:printcolumn:name="LaunchTemplate ID",type="string",JSONPath=".status.launchTemplateID",description="Launch Template ID"

// GCPMachinePool is the Schema for the gcpmachinepools API.
type GCPMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPMachinePoolSpec   `json:"spec,omitempty"`
	Status GCPMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:unservedversion

// GCPMachinePoolList contains a list of GCPMachinePool.
type GCPMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPMachinePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GCPMachinePool{}, &GCPMachinePoolList{})
}

// GetConditions returns the observations of the operational state of the GCPMachinePool resource.
func (r *GCPMachinePool) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the GCPMachinePool to the predescribed clusterv1.Conditions.
func (r *GCPMachinePool) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// GetObjectKind will return the ObjectKind of an GCPMachinePool.
func (r *GCPMachinePool) GetObjectKind() schema.ObjectKind {
	return &r.TypeMeta
}

// GetObjectKind will return the ObjectKind of an GCPMachinePoolList.
func (r *GCPMachinePoolList) GetObjectKind() schema.ObjectKind {
	return &r.TypeMeta
}
