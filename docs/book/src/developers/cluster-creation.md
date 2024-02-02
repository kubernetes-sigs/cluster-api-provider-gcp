# Creating cluster without clusterctl

This document describes how to create a management cluster and workload cluster without using clusterctl.
For creating a cluster with clusterctl, checkout our [Cluster API Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start.html)

## For creating a Management cluster

1. Build required images by using the following commands:

   - `docker build --tag=gcr.io/k8s-staging-cluster-api-gcp/cluster-api-gcp-controller:e2e .`
   - `make docker-build-all`

2. Set the required environment variables. For example:

   ```sh
   export GCP_REGION=us-east4
   export GCP_PROJECT=k8s-staging-cluster-api-gcp
   export CONTROL_PLANE_MACHINE_COUNT=1
   export WORKER_MACHINE_COUNT=1
   export KUBERNETES_VERSION=1.21.6
   export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
   export GCP_NODE_MACHINE_TYPE=n1-standard-2
   export GCP_NETWORK_NAME=default
   export GCP_B64ENCODED_CREDENTIALS=$( cat /path/to/gcp_credentials.json | base64 | tr -d '\n' )
   export CLUSTER_NAME="capg-test"
   export IMAGE_ID=projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-v1-27-3-nightly
   ```

  You can check for other images to set the `IMAGE_ID` of your choice.

3. Run `make create-management-cluster` from root directory.

