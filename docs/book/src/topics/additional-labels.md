# Additional Labels

The `additionalLabels` field lets you attach arbitrary GCP resource labels to
infrastructure objects managed by CAPG.

## Supported resources

| CRD | Labels applied to |
|-----|-------------------|
| `GCPCluster` | Load balancer forwarding rules, disks |
| `GCPMachine` | Compute Engine instances and their root disks |
| `GCPManagedCluster` | GKE cluster (`ResourceLabels`) |
| `GCPManagedMachinePool` | GKE node pool (`ResourceLabels`) |
| `GCPMachinePool` | Managed instance group instances |

## Usage

Set `additionalLabels` under `spec` on the relevant infrastructure object.
Label keys and values must conform to
[GCP label requirements](https://cloud.google.com/resource-manager/docs/creating-managing-labels#requirements).

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedCluster
metadata:
  name: my-cluster
spec:
  additionalLabels:
    env: production
    team: platform
```

## Precedence

When both a `GCPCluster`/`GCPManagedCluster` and a machine-level object
(`GCPMachine`, `GCPMachinePool`) define the same key, the **machine-level
value takes precedence**.

## Semantics for GKE clusters

For `GCPManagedCluster`, label management is **opt-in**. The controller only
reconciles `ResourceLabels` on the GKE cluster when `additionalLabels` is
explicitly set. Clusters with no `additionalLabels` field are left untouched,
so any labels applied directly in GCP are not disturbed.

Once opted in, `additionalLabels` is treated as the **complete desired label
set**. The GKE `SetLabels` API replaces all user-defined `ResourceLabels`, so
labels applied outside of CAPI that are not present in the spec will be
removed on the next reconcile. This is consistent with CAPI's source-of-truth
approach.

Label changes to an existing cluster are applied on the next reconcile cycle
after any pending cluster updates have completed, since GKE does not permit
concurrent cluster operations.
