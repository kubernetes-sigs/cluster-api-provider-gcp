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
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const (
	// ClusterFinalizer allows clean up GCP resources associated with GCPManagedCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "gcpmanagedcluster.infrastructure.cluster.x-k8s.io"
)

// GCPManagedClusterSpec defines the desired state of GCPManagedCluster.
type GCPManagedClusterSpec struct {
	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`

	// The GCP Region the cluster lives in.
	Region string `json:"region"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// NetworkSpec encapsulates all things related to the GCP network.
	// +optional
	Network infrav1.NetworkSpec `json:"network"`

	// AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalLabels infrav1.Labels `json:"additionalLabels,omitempty"`

	// ResourceManagerTags is an optional set of tags to apply to GCP resources managed
	// by the GCP provider. GCP supports a maximum of 50 tags per resource.
	// +maxItems=50
	// +optional
	ResourceManagerTags infrav1.ResourceManagerTags `json:"resourceManagerTags,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not
	// supplied then the credentials of the controller will be used.
	// +optional
	CredentialsRef *infrav1.ObjectReference `json:"credentialsRef,omitempty"`

	// LoadBalancerSpec contains configuration for one or more LoadBalancers.
	// +optional
	LoadBalancer infrav1.LoadBalancerSpec `json:"loadBalancer,omitempty"`

	// ServiceEndpoints contains the custom GCP Service Endpoint urls for each applicable service.
	// For instance, the user can specify a new endpoint for the compute service.
	// +optional
	ServiceEndpoints *infrav1.ServiceEndpoints `json:"serviceEndpoints,omitempty"`
}

// GCPManagedClusterStatus defines the observed state of GCPManagedCluster.
type GCPManagedClusterStatus struct {
	// +optional
	FailureDomains clusterv1beta1.FailureDomains `json:"failureDomains,omitempty"`
	Network        infrav1.Network               `json:"network,omitempty"`
	Ready          bool                          `json:"ready"`
	// conditions specifies the conditions for the managed cluster
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in GCPManagedCluster's status with the v1beta2 contract.
	// +optional
	V1Beta2 *GCPManagedClusterV1Beta2Status `json:"v1beta2,omitempty"`
}

// GCPManagedClusterV1Beta2Status groups the v1beta2 fields of GCPManagedClusterStatus.
type GCPManagedClusterV1Beta2Status struct {
	// conditions represents the observations of a GCPManagedCluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=gcpmanagedclusters,scope=Namespaced,categories=cluster-api,shortName=gcpmc
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this GCPCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for GCE instances"
// +kubebuilder:printcolumn:name="Network",type="string",JSONPath=".spec.network.name",description="GCP network the cluster is using"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.apiEndpoints[0]",description="API Endpoint",priority=1

// GCPManagedCluster is the Schema for the gcpmanagedclusters API.
type GCPManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPManagedClusterSpec   `json:"spec,omitempty"`
	Status GCPManagedClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GCPManagedClusterList contains a list of GCPManagedCluster.
type GCPManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPManagedCluster `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (r *GCPManagedCluster) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the status conditions for the GCPManagedCluster.
func (r *GCPManagedCluster) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for this object.
func (r *GCPManagedCluster) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the conditions on this object.
func (r *GCPManagedCluster) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &GCPManagedClusterV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&GCPManagedCluster{}, &GCPManagedClusterList{})
}
