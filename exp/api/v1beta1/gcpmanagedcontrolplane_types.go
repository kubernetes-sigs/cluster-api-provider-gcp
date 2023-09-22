/*
Copyright 2022 The Kubernetes Authors.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ManagedControlPlaneFinalizer allows Reconcile to clean up GCP resources associated with the GCPManagedControlPlane before
	// removing it from the apiserver.
	ManagedControlPlaneFinalizer = "gcpmanagedcontrolplane.infrastructure.cluster.x-k8s.io"
)

// GCPManagedControlPlaneSpec defines the desired state of GCPManagedControlPlane.
type GCPManagedControlPlaneSpec struct {
	// ClusterName allows you to specify the name of the GKE cluster.
	// If you don't specify a name then a default name will be created
	// based on the namespace and name of the managed control plane.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`
	// Location represents the location (region or zone) in which the GKE cluster
	// will be created.
	Location string `json:"location"`
	// ClusterIpv4Cidr is the IP address range of the container pods in the GKE cluster, in
	// [CIDR](http://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)
	// notation (e.g. `10.96.0.0/14`).
	// If not specified then one will be automatically chosen.
	// If this field is specified then IPAllocationPolicy.ClusterIpv4CidrBlock should be left blank.
	// +optional
	ClusterIpv4Cidr *string `json:"clusterIpv4Cidr,omitempty"`
	// EnableAutopilot indicates whether to enable autopilot for this GKE cluster.
	// +optional
	EnableAutopilot bool `json:"enableAutopilot"`
	// ReleaseChannel represents the release channel of the GKE cluster.
	// +optional
	ReleaseChannel *ReleaseChannel `json:"releaseChannel,omitempty"`
	// ControlPlaneVersion represents the control plane version of the GKE cluster.
	// If not specified, the default version currently supported by GKE will be
	// used.
	// +optional
	ControlPlaneVersion *string `json:"controlPlaneVersion,omitempty"`
	// Endpoint represents the endpoint used to communicate with the control plane.
	// +optional
	Endpoint clusterv1.APIEndpoint `json:"endpoint"`
	// AddonsConfig represents the configuration options for GKE cluster add ons.
	// +optional
	AddonsConfig *AddonsConfig `json:"addonsConfig,omitempty"`
	// LoggingConfig represents the configuration options for GKE cluster logging.
	// If not specified, none of the logging components are enabled.
	// +optional
	LoggingConfig *LoggingConfig `json:"loggingConfig,omitempty"`
	// MasterAuthorizedNetworksConfig represents configuration options for master authorized networks feature of the GKE cluster.
	// This feature is disabled if this field is not specified.
	// +optional
	MasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"masterAuthorizedNetworksConfig,omitempty"`
	// NetworkConfig represents configurations for GKE cluster networking
	// +optional
	NetworkConfig *NetworkConfig `json:"networkConfig,omitempty"`
	// PrivateClusterConfig represents configuration options for GKE private cluster feature.
	// This feature (both PrivateNodes and PrivateEndpoints) is disabled if this field is not specified.
	// +optional
	PrivateClusterConfig *PrivateClusterConfig `json:"privateClusterConfig,omitempty"`
	// WorkloadIdentityConfig represents configuration options for the use of Kubernetes Service Accounts in GCP IAM
	// policies. This feature is diabled if this field is not specified.
	// +optional
	WorkloadIdentityConfig *WorkloadIdentityConfig `json:"workloadIdentityConfig,omitempty"`
	// ResourceLabels represents the resource labels for the GKE cluster to use to annotate any related
	// Google Compute Engine resources.
	// +optional
	ResourceLabels map[string]string `json:"resourceLabels,omitempty"`
	// IPAllocationPolicy represents configuration options for GKE cluster IP allocation.
	// If not specified then GKE default values will be used.
	// +optional
	IPAllocationPolicy *IPAllocationPolicy `json:"ipAllocationPolicy,omitempty"`
	// MaintenancePolicy represents configuration options for the GKE cluster maintenance policy
	// +optional
	MaintenancePolicy *MaintenancePolicy `json:"maintenancePolicy,omitempty"`
	// DefaultMaxPodsConstraint represents the default constraint on the max number of pods that can be run
	// simultaneiously on a given node in a node pool of the GKE cluster.
	// Note: only honored if the GKE cluster is created with IPAllocationPolicy.UseIPAliases set to true.
	// +optional
	DefaultMaxPodsConstraint *MaxPodsConstraint `json:"defaultMaxPodsConstraint,omitempty"`
	// ShieldedNodes represents the configuration options for GKE ShieldedNodes feature
	// For Autopilot Enabled Clusters, ShieldedNodes is the default configuration.
	// Otherwise if not specified this feature is enabled by default on GKE 1.18+.
	// +optional
	ShieldedNodes *ShieldedNodes `json:"shieldedNodes,omitempty"`
}

// GCPManagedControlPlaneStatus defines the observed state of GCPManagedControlPlane.
type GCPManagedControlPlaneStatus struct {
	// Ready denotes that the GCPManagedControlPlane API Server is ready to
	// receive requests.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Initialized is true when the control plane is available for initial contact.
	// This may occur before the control plane is fully ready.
	// +optional
	Initialized bool `json:"initialized,omitempty"`

	// Conditions specifies the conditions for the managed control plane
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// CurrentVersion shows the current version of the GKE control plane.
	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=gcpmanagedcontrolplanes,scope=Namespaced,categories=cluster-api,shortName=gcpmcp
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this GCPManagedControlPlane belongs"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Control plane is ready"
// +kubebuilder:printcolumn:name="CurrentVersion",type="string",JSONPath=".status.currentVersion",description="The current Kubernetes version"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.endpoint",description="API Endpoint",priority=1

// GCPManagedControlPlane is the Schema for the gcpmanagedcontrolplanes API.
type GCPManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status GCPManagedControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GCPManagedControlPlaneList contains a list of GCPManagedControlPlane.
type GCPManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPManagedControlPlane `json:"items"`
}

// MaxPodsConstraint represents the constraint on the max number of pods that can be run
// simultaneously on a given node in a node pool of the GKE cluster.
type MaxPodsConstraint struct {
	// MaxPodsPerNode represents the constraint enforced on the max num of pods per node.
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:validation:Maximum=256
	MaxPodsPerNode int64 `json:"maxPodsPerNode,omitempty"`
}

// ReleaseChannel is the release channel of the GKE cluster
// +kubebuilder:validation:Enum=rapid;regular;stable
type ReleaseChannel string

const (
	// Rapid release channel.
	Rapid ReleaseChannel = "rapid"
	// Regular release channel.
	Regular ReleaseChannel = "regular"
	// Stable release channel.
	Stable ReleaseChannel = "stable"
)

// ShieldedNodes represents the configuration options for GKE ShieldedNodes feature.
type ShieldedNodes struct {
	// Enabled defines whether or not ShieldedNodes is enabled for the GKE cluster.
	Enabled bool `json:"enabled,omitempty"`
}

// IPAllocationPolicy represents configuration options for GKE cluster IP allocation.
type IPAllocationPolicy struct {
	// UseIPAliases represents whether alias IPs will be used for pod IPs in the cluster.
	// If unspecified will default to false.
	// +optional
	UseIPAliases *bool `json:"useIPAliases,omitempty"`
	// ClusterSecondaryRangeName represents the name of the secondary range to be used for the GKE cluster CIDR block.
	// The range will be used for pod IP addresses and must be an existing secondary range associated with the cluster subnetwork.
	// This field is only applicable when use_ip_aliases is set to true.
	// +optional
	ClusterSecondaryRangeName *string `json:"clusterSecondaryRangeName,omitempty"`
	// ServicesSecondaryRangeName represents the name of the secondary range to be used for the services CIDR block.
	// The range will be used for service ClusterIPs and must be an existing secondary range associated with the cluster subnetwork.
	// This field is only applicable when use_ip_aliases is set to true.
	// +optional
	ServicesSecondaryRangeName *string `json:"servicesSecondaryRangeName,omitempty"`
	// ClusterIpv4CidrBlock represents the IP address range for the GKE cluster pod IPs. If this field is set, then
	// GCPManagedControlPlaneSpec.ClusterIpv4Cidr must be left blank.
	// This field is only applicable when use_ip_aliases is set to true.
	// If not specified the range will be chosen with the default size.
	// +optional
	ClusterIpv4CidrBlock *string `json:"clusterIpv4CidrBlock,omitempty"`
	// ServicesIpv4CidrBlock represents the IP address range for services IPs in the GKE cluster.
	// This field is only applicable when use_ip_aliases is set to true.
	// If not specified the range will be chosen with the default size.
	// +optional
	ServicesIpv4CidrBlock *string `json:"servicesIpv4CidrBlock,omitempty"`
}

// MaintenancePolicy represents configuration options for the GKE cluster maintenance policy.
type MaintenancePolicy struct {
	// DailyMaintenanceWindow represents a daily maintenance time window for use with the GKE cluster.
	// Only one of DailyMaintenanceWindow and RecurringMaintenanceWindow can be set.
	// +optional
	DailyMaintenanceWindow *DailyMaintenanceWindow `json:"dailyMaintenanceWindow,omitempty"`
	// RecurringMaintenanceWindow represents a recurring maintenance time window for use with the GKE cluster.
	// Only one of DailyMaintenanceWindow and RecurringMaintenanceWindow can be set.
	// +optional
	RecurringMaintenanceWindow *RecurringMaintenanceWindow `json:"recurringMaintenanceWindow,omitempty"`
	// MaintenanceExclusions represents a map of maintenance exclusion names to a TimeWindow representing the exclusion period
	// (consists of a start time, end time, and exclusion scope). Non-emergency maintenance should not occur in these windows.
	// A maximum of 3 maintenance exclusions with "no-upgrades" is allowed.
	// +kubebuilder:validation:MaxProperties=20
	// +optional
	MaintenanceExclusions map[string]*TimeWindow `json:"maintenanceExclusions,omitempty"`
}

// DailyMaintenanceWindow represents a time window specified for daily maintenance operations.
type DailyMaintenanceWindow struct {
	// StartTime represents time within the maintenance window to start the maintenance operations.
	// Time format should be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt)
	// format "HH:MM", where HH : [00-23] and MM : [00-59] GMT.
	// +kubebuilder:validation:Pattern=`^([01]\d|2[0-3]):([0-5]\d)$`
	StartTime string `json:"startTime,omitempty"`
}

// RecurringMaintenanceWindow represents an arbitrary window of time that recurs.
type RecurringMaintenanceWindow struct {
	// Window represents the time window of first recurrence
	// +optional
	Window *TimeWindow `json:"window,omitempty"`
	// Recurrence represents a RRULE (https://tools.ietf.org/html/rfc5545#section-3.8.5.3) for how
	// the given window recurs. They go on for the span of time between the start and end time.
	// For more detail see (https://pkg.go.dev/cloud.google.com/go/container/apiv1/containerpb#RecurringTimeWindow)
	Recurrence string `json:"recurrence,omitempty"`
}

// TimeWindow represents an arbitrary window of time.
type TimeWindow struct {
	// StartTime represents the time that the window first starts.
	// Time format should follow Internet date/time format in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt)
	// i.e. "2006-01-02T15:04:05Z"
	StartTime string `json:"startTime,omitempty"`
	// EndTime represents the time that the window ends. The end time must take place after
	// the start time.
	// Time format should follow Internet date/time format in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt)
	// i.e. "2006-01-02T15:04:05Z"
	EndTime string `json:"endTime,omitempty"`
	// MaintenanceExclusionOption represents the maintenance exclusion scope for which
	// upgrades are blocked by the exclusion.
	// +optional
	MaintenanceExclusionOption *MaintenanceExclusionOption `json:"maintenanceExclusionOption,omitempty"`
}

// MaintenanceExclusionOption represents the maintenance exclusion options for a TimeWindow
// +kubebuilder:validation:Enum=no-upgrades;no-minor-upgrades;no-minor-or-node-upgrades
type MaintenanceExclusionOption string

const (
	// NoUpgrades excludes all upgrades, including patch upgrades and minor upgrades across
	// control planes and nodes. This is the default exclusion behavior.
	NoUpgrades MaintenanceExclusionOption = "no-upgrades"
	// NoMinorUpgrades excludes all minor upgrades for the GKE cluster, only patches are allowed.
	NoMinorUpgrades MaintenanceExclusionOption = "no-minor-upgrades"
	// NoMinorOrNodeUpgrades excludes all minor upgrades for the cluster, and also excludes
	// all node pool upgrades. Only control plane patches are allowed.
	NoMinorOrNodeUpgrades MaintenanceExclusionOption = "no-minor-or-node-upgrades"
)

// AddonsConfig contains configurations for various add ons available to run in the GKE cluster.
type AddonsConfig struct {
	// DNSCacheConfig represents a configuration for NodeLocalDNS, a dns cache running on GKE cluster nodes
	// If omitted it is disabled by default.
	// +optional
	DNSCacheConfig *DNSCacheConfig `json:"dnsCacheConfig,omitempty"`
	// GcePersistentDiskCsiDriverConfig represents a configuration for the Compute Engine Persistent Disk CSI driver
	// If omitted it is enabled by default.
	// +optional
	GcePersistentDiskCsiDriverConfig *GcePersistentDiskCsiDriverConfig `json:"gcePersistentDiskCsiDriverConfig,omitempty"`
}

// DNSCacheConfig contains configurations for NodeLocalDNS, a dns cache running on cluster nodes.
type DNSCacheConfig struct {
	// Enabled defines whether or not NodeLocal DNSCache is enabled for the GKE cluster
	Enabled bool `json:"enabled,omitempty"`
}

// DNSConfig represents the cluster DNS configurations for the GKE cluster.
type DNSConfig struct {
	// ClusterDNS represents which in-cluster DNS provider should be used.
	// If not specified will default to the GKE default DNS provider (kube-dns).
	// +optional
	ClusterDNS *ClusterDNS `json:"clusterDNS,omitempty"`
	// ClusterDNSScope represents the scope of access to the GKE cluster DNS records
	// If not specified will default to cluster scope.
	// +optional
	ClusterDNSScope *ClusterDNSScope `json:"clusterDNSScope,omitempty"`
	// ClusterDNSDomain is the suffix used for all GKE cluster service records
	// +optional
	ClusterDNSDomain *string `json:"clusterDNSDomain,omitempty"`
}

// ClusterDNS represents the in-cluster DNS provider
// +kubebuilder:validation:Enum=platform;cloud-dns;kube-dns
type ClusterDNS string

const (
	// PlatformDefault represents the GKE default DNS provider for DNS resolution.
	PlatformDefault ClusterDNS = "platform"
	// CloudDNS represents CloudDNS provider for DNS resolution.
	CloudDNS ClusterDNS = "cloud-dns"
	// KubeDNS represents KubeDNS provider for DNS resolution.
	KubeDNS ClusterDNS = "kube-dns"
)

// ClusterDNSScope represents the scope of access to GKE cluster DNS records.
// +kubebuilder:validation:Enum=cluster;vpc
type ClusterDNSScope string

const (
	// ClusterScope scopes DNS records to be accessible from within the cluster.
	ClusterScope ClusterDNSScope = "cluster"
	// VpcScope scopes DNS records to be accessible from within the VPC.
	VpcScope ClusterDNSScope = "vpc"
)

// GcePersistentDiskCsiDriverConfig contains configurations for the Compute Engine Persistent Disk CSI driver.
type GcePersistentDiskCsiDriverConfig struct {
	// Enabled defines whether or not the Compute Engine PD CSI driver is enabled for the GKE cluster.
	Enabled bool `json:"enabled,omitempty"`
}

// LoggingConfig represents the configurations for logging on the GKE cluster.
type LoggingConfig struct {
	// EnableComponents represents the components to collect logs from.
	// If left empty, logging on all components is disabled.
	// If non-empty, SystemComponents must be specified (will be added by default if not).
	// +optional
	EnableComponents []LoggingComponent `json:"enableComponents,omitempty"`
}

// LoggingComponent represents the component to enable logging for within the GKE cluster.
// +kubebuilder:validation:Enum=system-components;workloads;apiserver;scheduler;controller-manager
type LoggingComponent string

const (
	// SystemComponents represents logging for GKE system components.
	SystemComponents LoggingComponent = "system-components"
	// Workloads represents logging for Kubernetes workloads.
	Workloads LoggingComponent = "workloads"
	// APIServer represents logging for the kube-apiserver.
	APIServer LoggingComponent = "apiserver"
	// Scheduler represents logging for the kube-scheduler.
	Scheduler LoggingComponent = "scheduler"
	// ControllerManager represents logging for the kube-controller-manager.
	ControllerManager LoggingComponent = "controller-manager"
)

// MasterAuthorizedNetworksConfig contains configuration options for the master authorized networks feature.
// Enabled master authorized networks will disallow all external traffic to access
// Kubernetes master through HTTPS except traffic from the given CIDR blocks,
// Google Compute Engine Public IPs and Google Prod IPs.
type MasterAuthorizedNetworksConfig struct {
	// CidrBlocks define up to 50 external networks that could access
	// Kubernetes master through HTTPS.
	// +optional
	CidrBlocks []*MasterAuthorizedNetworksConfigCidrBlock `json:"cidrBlocks,omitempty"`
	// GcpPublicCidrsAccessEnabled defines whether master is accessible via Google Compute Engine Public IP addresses.
	// If PrivateClusterConfig PrivateEndpoint is enabled then this must be set to false or unspecified.
	// +optional
	GcpPublicCidrsAccessEnabled *bool `json:"gcpPublicCidrsAccessEnabled,omitempty"`
}

// MasterAuthorizedNetworksConfigCidrBlock contains an optional name and one CIDR block.
type MasterAuthorizedNetworksConfigCidrBlock struct {
	// DisplayName is an field for users to identify CIDR blocks.
	DisplayName string `json:"displayName,omitempty"`
	// CidrBlock must be specified in CIDR notation.
	// +kubebuilder:validation:Pattern=`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?:\/([0-9]|[1-2][0-9]|3[0-2]))?$|^([a-fA-F0-9:]+:+)+[a-fA-F0-9]+\/[0-9]{1,3}$`
	CidrBlock string `json:"cidrBlock,omitempty"`
}

// NetworkConfig represents configurations for GKE cluster networking.
type NetworkConfig struct {
	// DatapathProvider represents the desired datapath Provider for the GKE cluster.
	// If unspecified will use the default IPTables-based kube-proxy implementation.
	// +optional
	DatapathProvider *DatapathProvider `json:"datapathProvider,omitempty"`
	// DNSConfig represents the cluster DNS configurations for the GKE cluster.
	// +optional
	DNSConfig *DNSConfig `json:"dnsConfig,omitempty"`
}

// DatapathProvider represents the desired datapath Provider for the GKE cluster.
// The provider selects the implementation of the Kubernetes networking model for service resolution and network policy.
// +kubebuilder:validation:Enum=legacy;advanced
type DatapathProvider string

const (
	// LegacyDatapath uses the IPTables implementation based on kube-proxy.
	LegacyDatapath DatapathProvider = "legacy"
	// AdvancedDatapath uses the eBPF based GKE Dataplane V2 with additional features. See the [GKE Dataplane V2
	// documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/dataplane-v2) for more.
	AdvancedDatapath DatapathProvider = "advanced"
)

// PrivateClusterConfig contains configuration options for the Private Cluster feature.
// Private Clusters are VPC-native cluster where nodes only have internal IP addresses, meaning
// that nodes and Pods are isolated from the internet by default.
type PrivateClusterConfig struct {
	// Whether nodes have internal IP addresses only. If enabled, all nodes are
	// given only RFC 1918 private addresses and communicate with the master via
	// private networking.
	EnablePrivateNodes bool `json:"enablePrivateNodes,omitempty"`
	// Whether the master's internal IP address is used as the cluster endpoint.
	// If EnablePrivateEndpoint is true then MasterAuthorizedNetworksConfig must be set.
	EnablePrivateEndpoint bool `json:"enablePrivateEndpoint,omitempty"`
	// The IP range in CIDR notation to use for the hosted master network. This
	// range will be used for assigning internal IP addresses to the master or
	// set of masters, as well as the ILB VIP. This range must not overlap with
	// any other ranges in use within the cluster's network.
	// +optional
	MasterIpv4CidrBlock *string `json:"masterIpv4CidrBlock,omitempty"`
	// PrivateClusterMasterGlobalAccessEnabled controls if the master is globally accessible.
	// If not specified will default to false.
	// +optional
	PrivateClusterMasterGlobalAccessEnabled *bool `json:"privateClusterMasterGlobalAccessEnabled,omitempty"`
	// PrivateEndpointSubnetwork represents the subnet to provision the master's private endpoint during
	// cluster creation.
	// Specified in projects/*/regions/*/subnetworks/* format.
	// +kubebuilder:validation:Pattern=`^projects\/[a-z]([-a-z0-9]*[a-z0-9])?\/regions\/[a-z]([-a-z0-9]*[a-z0-9])?\/subnetworks\/[a-z]([-a-z0-9]*[a-z0-9])?`
	// +optional
	PrivateEndpointSubnetwork *string `json:"privateEndpointSubnetwork,omitempty"`
}

// WorkloadIdentityConfig contains configuration options for workload identity.
type WorkloadIdentityConfig struct {
	// WorkloadPool represents the node pool to attach all Kubernetes service accounts to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z0-9]*[a-z0-9])?`
	WorkloadPool string `json:"workloadPool,omitempty"`
}

// GetConditions returns the control planes conditions.
func (r *GCPManagedControlPlane) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the status conditions for the GCPManagedControlPlane.
func (r *GCPManagedControlPlane) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&GCPManagedControlPlane{}, &GCPManagedControlPlaneList{})
}
