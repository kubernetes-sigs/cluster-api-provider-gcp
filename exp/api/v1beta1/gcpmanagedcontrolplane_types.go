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
	// EnableAutopilot indicates whether to enable autopilot for this GKE cluster.
	EnableAutopilot bool `json:"enableAutopilot"`
	// Location represents the location (region or zone) in which the GKE cluster
	// will be created.
	Location string `json:"location"`
	// ReleaseChannel represents the release channel of the GKE cluster.
	// +optional
	ReleaseChannel *string `json:"releaseChannel,omitempty"`
	// ControlPlaneVersion represents the control plane version of the GKE cluster.
	// If not specified, the default version currently supported by GKE will be
	// used.
	// +optional
	ControlPlaneVersion *string `json:"controlPlaneVersion,omitempty"`
	// Endpoint represents the endpoint used to communicate with the control plane.
	Endpoint clusterv1.APIEndpoint `json:"endpoint"`
}

// GCPManagedControlPlaneStatus defines the observed state of GCPManagedControlPlane.
type GCPManagedControlPlaneStatus struct {
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=gcpmanagedcontrolplanes,scope=Namespaced,categories=cluster-api,shortName=gcpmcp
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

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

func init() {
	SchemeBuilder.Register(&GCPManagedControlPlane{}, &GCPManagedControlPlaneList{})
}
