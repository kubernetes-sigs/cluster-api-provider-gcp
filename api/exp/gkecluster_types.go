package exp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
)

const (
	// ClusterFinalizer allows ReconcileGCPCluster to clean up GCP resources associated with GCPCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "gcpcluster.infrastructure.cluster.x-k8s.io"
)

// GCPClusterSpec defines the desired state of GCPCluster.
type GKEClusterSpec struct {
	// Project is the name of the project to deploy the cluster to.
	Project string `json:"project"`

	// The GCP Region the cluster lives in.
	Region string `json:"region"`
}

// GKEClusterStatus defines the observed state of GCPCluster.
type GKEClusterStatus struct {
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
	//Network        Network                  `json:"network,omitempty"`

	// Bastion Instance `json:"bastion,omitempty"`
	Ready bool `json:"ready"`
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
type GKECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKEClusterSpec   `json:"spec,omitempty"`
	Status GKEClusterStatus `json:"status,omitempty"`
}

func (G GKECluster) DeepCopyObject() runtime.Object {
	panic("implement me")
}

// +kubebuilder:object:root=true

// GCPClusterList contains a list of GCPCluster.
type GKEClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GKECluster `json:"items"`
}

func (G GKEClusterList) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func init() {
	infrav1.SchemeBuilder.Register(&GKECluster{}, &GKEClusterList{})
}
