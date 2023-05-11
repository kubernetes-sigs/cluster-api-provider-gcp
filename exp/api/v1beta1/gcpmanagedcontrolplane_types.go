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
