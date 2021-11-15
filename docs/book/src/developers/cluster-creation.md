# Creating cluster without clusterctl

This document describes how to create a management cluster and workload cluster without using clusterctl.
For creating a cluster with clusterctl, checkout our [Cluster API Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start.html)

## For creating a Management cluster

1. Build the capg controller image by running `make docker-build` from the root directory

2. Build the node image

3. Set the environement variables
