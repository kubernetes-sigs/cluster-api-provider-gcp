# GKE Workload Identity Federation Settings

Workload Identity Federation allows an easy way to grant GCP IAM roles to workloads executing inside a GKE cluster and
is Google's recommended mechanism for securing and managing how workloads on GKE access GCP services. For more
information on this topic see
[Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/workload-identity).

## Requirements
In order to use this feature Workload Identity must be enabled at the cluster-level. On Autopilot clusters this
is set automatically. If you're not using Autopilot this can be done via GCP or via the `GCPManagedControlPlane`
CustomResource as follows:

```yaml
kind: GCPManagedControlPlane
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "my-cluster-control-plane"
spec:
  project: "my-project"
  clusterSecurity:
    workloadIdentityConfig:
      workloadPool: "myproject.svc.id.goog"
...
```

Later changes to this value will be reconciled by CAPG, changes to this value are non-disruptive, though changes can
take ~10 minutes to apply. Changing this **does not** recreate nodes.

### Notes
* The value of `workloadPool` must be of the form `<project>.svc.id.goog`, otherwise it will be rejected by GKE.
* You can enable and disable this feature as required, however you cannot disable it if Node Pools are set
  to `GKE_METADATA` mode. GKE will reject this update if you try.
* If `workloadIdentityConfig` is unset CAPG expresses no opinion about the value of `workloadPool` it may well be set,
  or it may not be. The GKE API is authoritative at that point.

## Enabling on Node Pools

The GCP Provider for Cluster API allows configuration of the nodepools to use a specific type of metadata server, via
the `.spec.nodeSecurity.workloadMetadataMode` field on the `GCPManagedMachinePool` CustomResource.
As can be seen in the example below:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: "${CLUSTER_NAME}-mp-0"
spec:
  nodeSecurity:
    workloadMetadataMode: "GKE_METADATA"
...
```

The above example would also work with the `GCPManagedMachinePoolTemplate` CustomResource with appropriate changes.

**Note:** When this is enabled on a nodepool it will take effect immediately, potentially causing workload disruption
if your workloads rely on being able to use the service account that the node uses.

In addition, nodes are recreated to implement this change so there may be some workload disruption as pods are migrated
to new nodes during this process.

See [Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) for more
details.

## Valid Values
The two valid values for `workloadMetadataMode` are `GKE_METADATA` & `GCE_METADATA`.

* `GKE_METADATA` configures the node pool to route metadata requests to a pod running on each node via a Daemonset.
* `GCE_METADATA` on the other hand lets pods access the metadata server on the underlying VM.

If `workloadMetadataMode` is unset it simply means CAPG does not manage that field and so it could take either
value or be unset on the cluster itself. The only way to ensure a value is to choose from one of the two valid ones.

## Troubleshooting
In the vast majority of cases of troubleshooting the above, you should look in controller logs. In many cases
you will see the results of the GKE API calls and that will aid troubleshooting.
