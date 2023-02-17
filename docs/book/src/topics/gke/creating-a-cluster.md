# Creating a GKE cluster

New "gke" cluster templates have been created that you can use with `clusterctl` to create a GKE cluster.

To create a GKE cluster with a managed node group (a.k.a managed machine pool):

```bash
clusterctl generate cluster capi-gke-quickstart --flavor gke --worker-machine-count=3 > capi-gke-quickstart.yaml
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
