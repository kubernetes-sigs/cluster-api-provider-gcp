# GPU enabled Cluster

This document describes how to create a management cluster and workload cluster with GPU enabled.

## For creating a Management cluster

1. Build the GPU enabled image by using the following command:

```sh
TODO: Update when the GPU enabled images are merged into the main repository and available.
```

2. Set the required environment variables. For example:

```sh
export GCP_PROJECT_ID=<YOUR PROJECT ID>
export GOOGLE_APPLICATION_CREDENTIALS=<PATH TO GCP CREDENTIALS>
export GCP_B64ENCODED_CREDENTIALS=$( cat /path/to/gcp-credentials.json | base64 | tr -d '\n' )
export CLUSTER_TOPOLOGY=true
export GCP_REGION="<GPU TYPE SUPPORTED REGION>"
export GCP_PROJECT="<YOU GCP PROJECT NAME>"
export KUBERNETES_VERSION=1.22.9
export IMAGE_ID=projects/$GCP_PROJECT/global/images/<IMAGE ID>
export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
export GCP_NODE_MACHINE_TYPE=n1-standard-2
export GCP_NETWORK_NAME=default
export CLUSTER_NAME="<YOUR CLUSTER NAME>"
```

3. Initialize the GCP infrastructure

```sh
clusterctl init --infrastructure gcp
```

4. Generate workload cluster configuration

```sh
clusterctl generate $CLUSTER_NAME --kubernetes-version $KUBERNETES_VERSION > workload-cluster.yaml
```

If you want to create cluster from a custom template, for example, create a cluster with gpu customized configuration, run the following command:

```sh
clusterctl generate $CLUSTER_NAME -- from path/to/cluster-template.yaml --kubernetes-version $KUBERNETES_VERSION > workload-cluster.yaml
```

5. Apply workload cluster configuration

```sh
kubectl apply -f workload-cluster.yaml
```
6. View Cluster status

```sh
clusterctl describe cluster $CLUSTER_NAME
```
