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

**Note:** If you want to provision a [zonal cluster](#zonal-clusters) you will need to edit the generated YAML. Set `spec.location` on `GCPManagedControlPlane` to a zone and keep `GCPManagedCluster.spec.region` as the region.

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

## Zonal clusters

CAPG allows you to provision either regional or zonal clusters depending on the value passed to `spec.location` on the `GCPManagedControlPlane` resource. In addition, the `GCPManagedCluster.spec.region` field must be the region that the zone specified on the `GCPManagedControlPlane` belongs to. For example, if `GCPManagedCluster.spec.region` is `us-central1` then for a zonal cluster `spec.location` must be something like `us-central1-a`. If this doesn't happen the `Ready` condition will be set to false.

A zonal cluster runs one control plane replica in a single zone (say `us-central1-a`) and replica count on the `MachinePool` is used as is, unless `nodeLocations` is set. This is in contrast to regional clusters where the node count has to be a multiple of the number of zones (3 by default or the number of `nodeLocations`) so nodes are deployed across multiple zones. GCP restricts converting zonal clusters to regional clusters later.

The `nodeLocations` field can spread a zonal cluster's pools across extra zones in the same region. However, replicas must divide evenly across listed zones; otherwise, preflight checks will fail upon create.

**Note:** Zonal clusters cannot be used with Autopilot and attempts to configure a cluster like this will cause the `Ready` condition to become false on the `GCPManagedControlPlane` object and any retries will not progress.

## Location resolution

Locations in this context refer to regions or zones (such as `us-central1` or `us-west2-a`), when these are set on CAPG resources the CAPG controller will resolve them against GCP. This means we can easily support new zones and regions as GCP brings them online rather than maintaining a list that we support.

This has the consequence that it's possible to set invalid locations, if this happens the `Ready` condition on the `GCPManagedControlPlane` object is set to false and a warning event is emitted. As locations are immutable, once this condition is set it will not retry, so recreate the edited object to fix this issue.

## VPC-native clusters: named secondary IP ranges

VPC-native GKE clusters (those with `useIPAliases: true`) can source pod and service IPs from named secondary ranges on the cluster's subnet, rather than specifying CIDR blocks directly. This is useful when subnets are pre-allocated and shared across multiple clusters.

Set `secondaryRangeName` on the `pod` and/or `service` blocks of `clusterNetwork`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-gke-control-plane
spec:
  project: my-gcp-project
  location: us-central1
  clusterNetwork:
    useIPAliases: true
    pod:
      secondaryRangeName: pods-range
    service:
      secondaryRangeName: services-range
```

The named ranges must already exist on the subnet before the cluster is created. Both fields are optional and independent of each other. They are **immutable** after cluster creation — the GCP API does not allow changes to the IP allocation policy of an existing cluster.
