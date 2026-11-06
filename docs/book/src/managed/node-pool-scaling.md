# Node Pool Scaling

When using GKE with CAPG you can opt in to using Autoscaling via the GKE Autoscaler for your node pools. To enable
this add a `scaling` block to your `GCPManagedMachinePool` resource as below. You can also configure this with the
`GCPManagedMachinePoolTemplate` resource if required:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: np-size-mp-0
  namespace: default
spec:
  scaling:
    enableAutoscaling: true
    minCount: 1
    maxCount: 6
```

## Configuring The Autoscaling

To further configure the autoscaling there are four fields that can be set:

- `minCount` - The minimum _total_ count of instances (across all zones) that the nodepool can scale down to
- `maxCount` - The maximum _total_ count of instances (across all zones) that the nodepool can scale up to
- `locationPolicy` - The name of the policy that dictates how the location of the creation/deletion of new nodes occurs
- `enableAutoscaling` - Turns the autoscaler on and off

**Note:** Older versions of CAPG turned on auto-scaling with a cap of 1 node for any pool with no scaling block. This is
  also true of setting `scaling` to `{}`. To change this either set `enableAutoscaling: false` or set `minCount` and
  `maxCount` to specific limits.

## How Is Size Decided?

Introducing the Autoscaler does mean that the semantics of `replicas` on the `MachinePool` associated with the
`GCPManagedMachinePool` change. Specifically there are three cases that matter:

1. No `scaling` block - This means that autoscaling is 'unmanaged' by CAPG, and thus replicas is treated as a command rather
  than a suggestion
2. A `scaling` that is enabled - This means that `replicas` is a starting size only and GKE dictates the size of
  the node pool
3. A `scaling` block with `enableAutoscaling: false` - This is the same as (1) but `minCount` and `maxCount` are ignored

## Tracking Replicas

As above there are situations where CAPG does not directly set the number of replicas per node pool. In those instances
`status.replicas` tracks the live count of replicas and CAPG sets `cluster.x-k8s.io/replicas-managed-by: gcp-autoscaler`,
which ensures that the correct statuses are displayed by the `MachinePool`

## Regional & Zonal Clusters

Autoscaling can be enabled on regional or zonal clusters and new replicas are divided amongst a pool's zones. This
occurs in Cases (1) and (3) above.
