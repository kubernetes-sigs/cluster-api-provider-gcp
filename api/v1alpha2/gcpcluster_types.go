/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterFinalizer allows ReconcileGCPCluster to clean up GCP resources associated with GCPCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "gcpcluster.infrastructure.cluster.x-k8s.io"
)

// GCPClusterSpec defines the desired state of GCPCluster
type GCPClusterSpec struct {
	// NetworkSpec encapsulates all things related to GCP network.
	NetworkSpec NetworkSpec `json:"networkSpec,omitempty"`

	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`

	// The GCP Region the cluster lives in.
	Region string `json:"region"`

	// The default zone to create instances in.
	// If empty, the first zone available in the Region is used.
	// +optional
	DefaultZone *string `json:"defaultZone,omitempty"`

	// The Network zone to create instances in.
	// If empty, the GCP default network is used.
	// +optional
	Network *string `json:"network,omitempty"`

	// AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalLabels Labels `json:"additionalLabels,omitempty"`
}

// GCPClusterStatus defines the observed state of GCPCluster
type GCPClusterStatus struct {
	Network Network `json:"network,omitempty"`

	// Bastion Instance `json:"bastion,omitempty"`
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=gcpclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// GCPCluster is the Schema for the gcpclusters API
type GCPCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPClusterSpec   `json:"spec,omitempty"`
	Status GCPClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GCPClusterList contains a list of GCPCluster
type GCPClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GCPCluster{}, &GCPClusterList{})
}
