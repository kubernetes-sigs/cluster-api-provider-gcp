# Provisioning a GKE cluster

<aside class="note warning">

<h1>Warning</h1>

We recommend you take a look at the [Prerequisites](./../prerequisites.md) section before provisioning a workload cluster. 

</aside>

**This guide uses an example from the `./templates` folder of the CAPG repository. You can inspect the yaml file [here](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/refs/heads/main/templates/cluster-template-gke.yaml).**

## Configure cluster parameters

While inspecting the cluster definition in `./templates/cluster-template-gke.yaml` you probably noticed that it contains a number of parameterized values that must be substituted with the specifics of your use case. This can be done via environment variables and `clusterctl` and effectively makes the template more flexible to adapt to different provisioning scenarios. These are the environment variables that you'll be required to set before deploying a workload cluster:

```sh
export GCP_PROJECT=cluster-api-gcp-project
export GCP_REGION=us-east4
export GCP_NETWORK_NAME=default
export WORKER_MACHINE_COUNT=1
```

## Generate cluster definition

The sample cluster templates are already prepared so that you can use them with `clusterctl` to create a GKE cluster with CAPG.

To create a GKE cluster with a managed node group (a.k.a managed machine pool):

```bash
clusterctl generate cluster capi-gke-quickstart --flavor gke -i gcp > capi-gke-quickstart.yaml
```

In this example, `capi-gke-quickstart` will be used as cluster name.

## Create cluster

The resulting file represents the workload cluster definition and you simply need to apply it to your cluster to trigger cluster creation:

```
kubectl apply -f capi-gke-quickstart.yaml
```

## Kubeconfig

When creating an GKE cluster 2 kubeconfigs are generated and stored as secrets in the management cluster.

### User kubeconfig

This should be used by users that want to connect to the newly created GKE cluster. The name of the secret that contains the kubeconfig will be `[cluster-name]-user-kubeconfig` where you need to replace **[cluster-name]** with the name of your cluster. The **-user-kubeconfig** in the name indicates that the kubeconfig is for the user use.

To get the user kubeconfig for a cluster named `managed-test` you can run a command similar to:

```bash
kubectl --namespace=default get secret managed-test-user-kubeconfig \
   -o jsonpath={.data.value} | base64 --decode \
   > managed-test.kubeconfig
```

### Cluster API (CAPI) kubeconfig

This kubeconfig is used internally by CAPI and shouldn't be used outside of the management server. It is used by CAPI to perform operations, such as draining a node. The name of the secret that contains the kubeconfig will be `[cluster-name]-kubeconfig` where you need to replace **[cluster-name]** with the name of your cluster. Note that there is NO `-user` in the name.

The kubeconfig is regenerated every `sync-period` as the token that is embedded in the kubeconfig is only valid for a short period of time.
