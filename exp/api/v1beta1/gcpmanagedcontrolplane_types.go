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

// PrivateCluster defines a private Cluster.
type PrivateCluster struct {
	// EnablePrivateEndpoint: Whether the master's internal IP
	// address is used as the cluster endpoint.
	// +optional
	EnablePrivateEndpoint bool `json:"enablePrivateEndpoint,omitempty"`

	// EnablePrivateNodes: Whether nodes have internal IP
	// addresses only. If enabled, all nodes are given only RFC
	// 1918 private addresses and communicate with the master via
	// private networking.
	// +optional
	EnablePrivateNodes bool `json:"enablePrivateNodes,omitempty"`

	// ControlPlaneCidrBlock is the IP range in CIDR notation to use for the hosted master network. This range must not
	// overlap with any other ranges in use within the cluster's network. Honored when enabled is true.
	// +optional
	ControlPlaneCidrBlock string `json:"controlPlaneCidrBlock,omitempty"`

	// ControlPlaneGlobalAccess is whenever master is accessible globally or not. Honored when enabled is true.
	// +optional
	ControlPlaneGlobalAccess bool `json:"controlPlaneGlobalAccess,omitempty"`

	// DisableDefaultSNAT is disables cluster default sNAT rules. Honored when enabled is true.
	// +optional
	DisableDefaultSNAT bool `json:"disableDefaultSNAT,omitempty"`
}

// ClusterNetworkPod the range of CIDRBlock list from where it gets the IP address.
type ClusterNetworkPod struct {
	// CidrBlock is where all pods in the cluster are assigned an IP address from this range. Enter a range
	// (in CIDR notation) within a network range, a mask, or leave this field blank to use a default range.
	// This setting is permanent.
	// +optional
	CidrBlock string `json:"cidrBlock,omitempty"`
}

// ClusterNetworkService defines the range of CIDRBlock list from where it gets the IP address.
type ClusterNetworkService struct {
	// CidrBlock is where cluster services will be assigned an IP address from this IP address range. Enter a range
	// (in CIDR notation) within a network range, a mask, or leave this field blank to use a default range.
	// This setting is permanent.
	// +optional
	CidrBlock string `json:"cidrBlock,omitempty"`
}

// ClusterNetwork define the cluster network.
type ClusterNetwork struct {
	// PrivateCluster defines the private cluster spec.
	// +optional
	PrivateCluster *PrivateCluster `json:"privateCluster,omitempty"`

	// UseIPAliases is whether alias IPs will be used for pod IPs in the cluster. If false, routes will be used for
	// pod IPs in the cluster.
	// +optional
	UseIPAliases bool `json:"useIPAliases,omitempty"`

	// Pod defines the range of CIDRBlock list from where it gets the IP address.
	// +optional
	Pod *ClusterNetworkPod `json:"pod,omitempty"`

	// Service defines the range of CIDRBlock list from where it gets the IP address.
	// +optional
	Service *ClusterNetworkService `json:"service,omitempty"`
}

// WorkloadIdentityConfig allows workloads in your GKE clusters to impersonate Identity and Access Management (IAM)
// service accounts to access Google Cloud services.
type WorkloadIdentityConfig struct {
	// WorkloadPool is the workload pool to attach all Kubernetes service accounts to Google Cloud services.
	// Only relevant when enabled is true
	// +kubebuilder:validation:Required
	WorkloadPool string `json:"workloadPool,omitempty"`
}

// AuthenticatorGroupConfig is RBAC security group for use with Google security groups in Kubernetes RBAC.
type AuthenticatorGroupConfig struct {
	// SecurityGroups is the name of the security group-of-groups to be used.
	// +kubebuilder:validation:Required
	SecurityGroups string `json:"securityGroups,omitempty"`
}

// ClusterSecurity defines the cluster security.
type ClusterSecurity struct {
	// WorkloadIdentityConfig allows workloads in your GKE clusters to impersonate Identity and Access Management (IAM)
	// service accounts to access Google Cloud services
	// +optional
	WorkloadIdentityConfig *WorkloadIdentityConfig `json:"workloadIdentityConfig,omitempty"`

	// AuthenticatorGroupConfig is RBAC security group for use with Google security groups in Kubernetes RBAC.
	// +optional
	AuthenticatorGroupConfig *AuthenticatorGroupConfig `json:"authenticatorGroupConfig,omitempty"`

	// EnableLegacyAuthorization Whether the legacy (ABAC) authorizer is enabled for this cluster.
	// +optional
	EnableLegacyAuthorization bool `json:"enableLegacyAuthorization,omitempty"`

	// IssueClientCertificate is weather to issue a client certificate.
	// +optional
	IssueClientCertificate bool `json:"issueClientCertificate,omitempty"`
}

// AddonsConfig defines the enabled Cluster Addons.
type AddonsConfig struct {
	// CloudRun enable the Cloud Run addon, which allows  the user to use a managed Knative service.
	// +optional
	CloudRun bool `json:"cloudRun,omitempty"`

	// KalmConfig enable the KALM addon, which manages the lifecycle of k8s applications.
	// +optional
	KalmConfig bool `json:"kalmConfig,omitempty"`

	// GKEBackup whether the Backup for GKE agent is enabled for this cluster.
	// +optional
	GKEBackup bool `json:"GKEBackup,omitempty"`

	// GCEPersistentDiskCsiDriver whether the Compute Engine PD CSI driver is enabled for this cluster.
	// +optional
	GCEPersistentDiskCsiDriver bool `json:"GCEPersistentDiskCsiDriver,omitempty"`

	// GCPFileStoreCsiDriver whether the GCP Filestore CSI driver is enabled for this cluster.
	// +optional
	GCPFileStoreCsiDriver bool `json:"GCPFileStoreCsiDriver,omitempty"`

	// ImageStreaming whether to use GCFS (Google Container File System).
	// +optional
	ImageStreaming bool `json:"ImageStreaming,omitempty"`
}

// LoggingConfig defines the logging on Cluster.
type LoggingConfig struct {
	// SystemComponents enables the system component logging.
	// +optional
	SystemComponents bool `json:"systemComponents,omitempty"`

	// Workloads enables the Workloads logging.
	// +optional
	Workloads bool `json:"workloads,omitempty"`

	// APIServer enables the api server logging.
	// +optional
	APIServer bool `json:"apiServer,omitempty"`

	// Scheduler enables the scheduler logging.
	// +optional
	Scheduler bool `json:"scheduler,omitempty"`

	// ControllerManager enables the controller manager logging.
	// +optional
	ControllerManager bool `json:"controllerManager,omitempty"`
}

// MonitoringConfig defines the monitoring on Cluster.
type MonitoringConfig struct {
	// SystemComponents enables the system component monitoring.
	// +optional
	SystemComponents bool `json:"systemComponents,omitempty"`

	// APIServer enables the api server monitoring.
	// +optional
	APIServer bool `json:"apiServer,omitempty"`

	// Scheduler enables the scheduler monitoring.
	// +optional
	Scheduler bool `json:"scheduler,omitempty"`

	// ControllerManager enables the controller manager monitoring.
	// +optional
	ControllerManager bool `json:"controllerManager,omitempty"`

	// EnableManagedPrometheus Enable Google Cloud Managed Service for Prometheus in the cluster.
	// +optional
	EnableManagedPrometheus bool `json:"enableManagedPrometheus,omitempty"`
}

// GCPManagedControlPlaneSpec defines the desired state of GCPManagedControlPlane.
type GCPManagedControlPlaneSpec struct {
	// ClusterName allows you to specify the name of the GKE cluster.
	// If you don't specify a name then a default name will be created
	// based on the namespace and name of the managed control plane.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Description describe the cluster.
	// +optional
	Description string `json:"description,omitempty"`

	// ClusterNetwork define the cluster network.
	// +optional
	ClusterNetwork *ClusterNetwork `json:"clusterNetwork,omitempty"`

	// ClusterSecurity defines the cluster security.
	// +optional
	ClusterSecurity *ClusterSecurity `json:"clusterSecurity,omitempty"`

	// AddonsConfig defines the enabled Cluster Addons.
	// +optional
	AddonsConfig *AddonsConfig `json:"addonsConfig,omitempty"`

	// LoggingConfig defines the logging on Cluster.
	// +optional
	LoggingConfig *LoggingConfig `json:"loggingConfig,omitempty"`

	// MonitoringConfig defines the monitoring on Cluster.
	// +optional
	MonitoringConfig *MonitoringConfig `json:"monitoringConfig,omitempty"`

	// DefaultNodeLocation is the list of Google Compute Engine zones in which the cluster's Node should be located.
	// +optional
	DefaultNodeLocation []string `json:"defaultNodeLocation,omitempty"`

	// DefaultMaXPodsPerNode is the maximum number of pods can be run simultaneously on a Node, and only honored if
	// Cluster is created with IP Alias support.
	// +optional
	DefaultMaxPodsPerNode int `json:"defaultMaxPodsPerNode,omitempty"`

	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`
	// Location represents the location (region or zone) in which the GKE cluster
	// will be created.
	Location string `json:"location"`
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
	// MasterAuthorizedNetworksConfig represents configuration options for master authorized networks feature of the GKE cluster.
	// This feature is disabled if this field is not specified.
	// +optional
	MasterAuthorizedNetworksConfig *MasterAuthorizedNetworksConfig `json:"master_authorized_networks_config,omitempty"`
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
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Control plane is ready"
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

// MasterAuthorizedNetworksConfig contains configuration options for the master authorized networks feature.
// Enabled master authorized networks will disallow all external traffic to access
// Kubernetes master through HTTPS except traffic from the given CIDR blocks,
// Google Compute Engine Public IPs and Google Prod IPs.
type MasterAuthorizedNetworksConfig struct {
	// cidr_blocks define up to 50 external networks that could access
	// Kubernetes master through HTTPS.
	// +optional
	CidrBlocks []*MasterAuthorizedNetworksConfigCidrBlock `json:"cidr_blocks,omitempty"`
	// Whether master is accessible via Google Compute Engine Public IP addresses.
	// +optional
	GcpPublicCidrsAccessEnabled *bool `json:"gcp_public_cidrs_access_enabled,omitempty"`
}

// MasterAuthorizedNetworksConfigCidrBlock contains an optional name and one CIDR block.
type MasterAuthorizedNetworksConfigCidrBlock struct {
	// display_name is an field for users to identify CIDR blocks.
	DisplayName string `json:"display_name,omitempty"`
	// cidr_block must be specified in CIDR notation.
	// +kubebuilder:validation:Pattern=`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?:\/([0-9]|[1-2][0-9]|3[0-2]))?$|^([a-fA-F0-9:]+:+)+[a-fA-F0-9]+\/[0-9]{1,3}$`
	CidrBlock string `json:"cidr_block,omitempty"`
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
