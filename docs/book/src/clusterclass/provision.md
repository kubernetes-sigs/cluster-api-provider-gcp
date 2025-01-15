# Provisioning a Cluster via ClusterClass

<aside class="note warning">

<h1>Warning</h1>

We recommend you take a look at the [Prerequisites](./../prerequisites.md) section before provisioning a workload cluster. 

**You should be familiar with the ClusterClass feature and its core concepts: [CAPI book on ClusterClass and Managed Topologies](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/write-clusterclass).**

</aside>


**This guide uses an example from the `./templates` folder of the CAPG repository. You can inspect the yaml file for the `ClusterClass` [here](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/refs/heads/main/templates/cluster-template-clusterclass.yaml) and the cluster definition [here](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/refs/heads/main/templates/cluster-template-topology.yaml).**


## Templates and clusters

ClusterClass makes cluster templates more flexible and versatile as it allows users to create cluster flavors that can be reused for cluster provisioning.

In this case, while inspecting the sample files, you probably noticed that there are references to two different yaml:
- `./templates/cluster-template-clusterclass.yaml` is the class definition. It represents the template that define a topology: control plane and machine deployment but it won't provision the cluster.
- `./templates/cluster-template-topology.yaml` is the cluster definition that references the class. This workload cluster definition is considerably simpler than a regular CAPI cluster template that does not use ClusterClass, as most of the complexity of defining the control plane and machine deployment has been removed by the class.

## Configure ClusterClass

While inspecting the templates you probably noticed that they contain a number of parameterized values that must be substituted with the specifics of your use case. This can be done via environment variables and `clusterctl` and effectively make the templates more flexible to adapt to different provisioning scenarios. These are the environment variables that you'll be required to set before deploying a class and a workload cluster from it:

```sh
export CLUSTER_CLASS_NAME=sample-cc
export GCP_PROJECT=cluster-api-gcp-project
export GCP_REGION=us-east4
export GCP_NETWORK_NAME=default
export IMAGE_ID=projects/cluster-api-gcp-project/global/images/your-image
```

## Generate ClusterClass definition

The sample ClusterClass template is already prepared so that you can use it with `clusterctl` to create a CAPI ClusterClass with CAPG.

```bash
clusterctl generate cluster capi-gcp-quickstart-clusterclass --flavor clusterclass -i gcp > capi-gcp-quickstart-clusterclass.yaml
```

In this example, `capi-gcp-quickstart-clusterclass` will be used as class name.

## Create ClusterClass

The resulting file represents the class template definition and you simply need to apply it to your cluster to make it available in the API:

```
kubectl apply -f capi-gcp-quickstart-clusterclass.yaml
```

## Create a cluster from a class

ClusterClass is a powerful feature of CAPI because we can now create one or multiple clusters that are based on the same class that is available in the CAPI Management Cluster. This base template can be parameterized so clusters created from it can make slight changes to the original configuration and adapt to the specifics of the use case, e.g. provisioning clusters for different development, staging and production environments.

Now that the class is available to be referenced by cluster objects, let's configure the workload cluster and provision it.

```sh
export CLUSTER_NAME=sample-cluster
export CLUSTER_CLASS_NAME=sample-cc
export KUBERNETES_VERSION=1.29.3
export CONTROL_PLANE_MACHINE_COUNT=1
export WORKER_MACHINE_COUNT=1
export GCP_REGION=us-east4
export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
export GCP_NODE_MACHINE_TYPE=n1-standard-2
export CNI_RESOURCES=./cni-resource
```

You can take a look at CAPG's CNI requirements [here](./../self-managed/cni.md)

You can use `clusterctl` to create a cluster definition.

```bash
clusterctl generate cluster capi-gcp-quickstart-topology --flavor topology -i gcp > capi-gcp-quickstart-topology.yaml
```

And by simply applying the resulting template, the cluster will be provisioned based on the existing ClusterClass.

```
kubectl apply -f capi-gcp-quickstart-topology.yaml
```

You can now experiment with creating more clusters based on this class while applying different configurations to each workload cluster.
