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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileGCPCluster to clean up GCP resources associated with GCPCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "gcpcluster.infrastructure.cluster.x-k8s.io"
)

// GCPClusterSpec defines the desired state of GCPCluster.
type GCPClusterSpec struct {
	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`

	// The GCP Region the cluster lives in.
	Region string `json:"region"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// NetworkSpec encapsulates all things related to GCP network.
	// +optional
	Network NetworkSpec `json:"network"`

	// FailureDomains is an optional field which is used to assign selected availability zones to a cluster
	// FailureDomains if empty, defaults to all the zones in the selected region and if specified would override
	// the default zones.
	// +optional
	FailureDomains []string `json:"failureDomains,omitempty"`

	// AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalLabels Labels `json:"additionalLabels,omitempty"`

	// ResourceManagerTags is an optional set of tags to apply to GCP resources managed
	// by the GCP provider. GCP supports a maximum of 50 tags per resource.
	// +maxItems=50
	// +optional
	ResourceManagerTags ResourceManagerTags `json:"resourceManagerTags,omitempty"`

	// CredentialsRef is a reference to a Secret that contains the credentials to use for provisioning this cluster. If not
	// supplied then the credentials of the controller will be used.
	// When creating a new GCP client, the controller will try to extract the type
	// of credential from the JSON data, and it will request a client for the specific credential type.
	// +optional
	CredentialsRef *ObjectReference `json:"credentialsRef,omitempty"`

	// LoadBalancer contains configuration for one or more LoadBalancers.
	// +optional
	LoadBalancer LoadBalancerSpec `json:"loadBalancer,omitempty"`

	// ServiceEndpoints contains the custom GCP Service Endpoint urls for each applicable service.
	// For instance, the user can specify a new endpoint for the compute service.
	// +optional
	ServiceEndpoints *ServiceEndpoints `json:"serviceEndpoints,omitempty"`
}

// GCPClusterStatus defines the observed state of GCPCluster.
type GCPClusterStatus struct {
	FailureDomains clusterv1beta1.FailureDomains `json:"failureDomains,omitempty"`
	Network        Network                       `json:"network,omitempty"`

	// Bastion Instance `json:"bastion,omitempty"`
	Ready bool `json:"ready"`

	// conditions defines current service state of the GCPCluster.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in GCPCluster's status
	// with the v1beta2 version of the Cluster API contract.
	// +optional
	V1Beta2 *GCPClusterV1Beta2Status `json:"v1beta2,omitempty"`
}

// GCPClusterV1Beta2Status groups the fields that will be added or modified in GCPCluster's status
// with the v1beta2 version of the Cluster API contract.
type GCPClusterV1Beta2Status struct {
	// conditions represents the observations of a GCPCluster's current state.
	// Known condition types are Ready.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=gcpclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this GCPCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for GCE instances"
// +kubebuilder:printcolumn:name="Network",type="string",JSONPath=".spec.network.name",description="GCP network the cluster is using"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.apiEndpoints[0]",description="API Endpoint",priority=1

// GCPCluster is the Schema for the gcpclusters API.
type GCPCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPClusterSpec   `json:"spec,omitempty"`
	Status GCPClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GCPClusterList contains a list of GCPCluster.
type GCPClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPCluster `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (c *GCPCluster) GetConditions() clusterv1beta1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *GCPCluster) SetConditions(conditions clusterv1beta1.Conditions) {
	c.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for this object.
func (c *GCPCluster) GetV1Beta2Conditions() []metav1.Condition {
	if c.Status.V1Beta2 == nil {
		return nil
	}
	return c.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the conditions on this object.
func (c *GCPCluster) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if c.Status.V1Beta2 == nil {
		c.Status.V1Beta2 = &GCPClusterV1Beta2Status{}
	}
	c.Status.V1Beta2.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&GCPCluster{}, &GCPClusterList{})
}
