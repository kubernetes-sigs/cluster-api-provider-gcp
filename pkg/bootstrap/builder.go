package bootstrap

import (
	compute "google.golang.org/api/compute/v1"
	gceconfigv1 "sigs.k8s.io/cluster-api-provider-gcp/pkg/apis/gceproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/google/machinesetup"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type MetadataBuilder interface {
	BuildMetadata(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clusterConfig *gceconfigv1.GCEClusterProviderSpec, configParams *machinesetup.ConfigParams) (*compute.Metadata, error)
}
