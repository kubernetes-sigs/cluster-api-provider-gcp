# Machine Pools

CAPG implements the CAPI `MachinePool` concept as the experimental `GCPMachinePool` CustomResource. This allows you
to manage sets of GCP VMs via a single resource rather than independently. This is backed by a zonal managed
instance group (MIG) and Instance Template in GCP. Due to its experimental nature it's off by default
and so must be enabled by a feature gate.

## Enabling Machine Pools

To enable Machine Pools you should run `export EXP_MACHINE_POOL=true` and then `clusterctl init`. This only needs
to be set for CAPG as CAPI's own feature gate is on by default.

## Defining a Pool

To define a pool you need to define three separate objects, `GCPMachinePool`, a `KubeadmConfig` and a `MachinePool` that
references the other two.

An example of each is shown below, first the `GCPMachinePool`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachinePool
metadata:
  name: "my-cluster-md-0"
spec:
  instanceType: "n2-standard-4"
  image: "projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-v1-35-1-nightly"
```

Next the `KubeadmConfig` (note this is **not** a `KubeadmConfigTemplate`), also note that because `cloud-provider` is
set to `external` you'll need to provide the external CCM in the cluster:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfig
metadata:
  name: "my-cluster-md-0"
spec:
  joinConfiguration:
    nodeRegistration:
      name: '{{ ds.meta_data.local_hostname.split(".")[0] }}'
      kubeletExtraArgs:
        cloud-provider: external
```

And finally the `MachinePool` that brings the two together:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: "my-cluster-md-0"
spec:
  clusterName: "my-cluster"
  replicas: 3
  failureDomains:
  - us-east4-a
  template:
    spec:
      clusterName: "my-cluster"
      version: "1.35.1"
      bootstrap:
        configRef:
          name: "my-cluster-md-0"
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfig
      infrastructureRef:
        name: "my-cluster-md-0"
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: GCPMachinePool
```

## Limitations

As the support for MachinePools is limited there are some limitations to its usage, only one GCP zone is permitted
per MachinePool (`failureDomains` in the example above). If `failureDomains` is omitted the pool falls back to every
failure domain of the cluster, which will cause the provisioning to fail with an error message like
`instanceGroupManager must be created in a single zone, got N zones...`. To fix this set a value in `failureDomains`.

It should also be noted that we do not support `MachinePool` machines at present.

## Scaling

Scaling a `MachinePool` is very easy, simply change the `spec.replicas` field on the `MachinePool`, and that translates
to the MIG target size. The controller then resizes the MIG which causes new instances to be spun up or terminated.

## Checking Health

Checking health can be done via `kubectl describe gcpmachinepool <name>`. Using this command you will be able to see
the conditions on the object. The ones of interest are `ManagedInstanceGroupReady`, which indicates if the underlying
MIG is ready, `InstanceTemplateReady` which does the same for the InstanceTemplate and the overall `Ready` condition.

It's also worth noting that early on in the creation of the MachinePool you may get errors in the logs like
`Bootstrap data secret reference is not yet available`. This is very common in the early stages of provisioning and
should not be a cause for concern.
