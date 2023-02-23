# GKE Support in the GCP Provider

- **Feature status:** Experimental
- **Feature gate (required):** GKE=true

## Overview

The GCP provider supports creating GKE based cluster. Currently the following features are supported:

- Provisioning/managing a GCP GKE Cluster
- Upgrading the Kubernetes version of the GKE Cluster
- Creating a managed node pool and attaching it to the GKE cluster

The implementation introduces the following CRD kinds:

- GCPManagedCluster - presents the properties needed to provision and manage the general GCP operating infrastructure for the cluster (i.e project, networking, iam)
- GCPManagedControlPlane - specifies the GKE Cluster in GCP and used by the Cluster API GCP Managed Control plane
- GCPManagedMachinePool - defines the managed node pool for the cluster

And a new template is available in the templates folder for creating a managed workload cluster.

## SEE ALSO

* [Enabling GKE Support](enabling.md)
* [Disabling GKE Support](disabling.md)
* [Creating a cluster](creating-a-cluster.md)
* [Cluster Upgrades](cluster-upgrades.md)