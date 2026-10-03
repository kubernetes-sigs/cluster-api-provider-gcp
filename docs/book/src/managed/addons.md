# Add-ons

`spec.addonsConfig` on `GCPManagedControlPlane` lets you explicitly enable or disable GKE add-ons. It's a map keyed by add-on name, where each entry says whether the add-on is enabled and, for the few add-ons that have any, sets its options:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  clusterSecurity:
    workloadIdentityConfig:
      workloadPool: my-project.svc.id.goog
  addonsConfig:
    gcsFuseCsiDriverConfig:
      enabled: true
    dnsCacheConfig:
      enabled: true
    rayOperatorConfig:
      enabled: true
      options:
        rayClusterLoggingConfig: true
```

Leaving an add-on out of the map entirely keeps GKE's own default for it in place — CAPG doesn't impose an opinion on add-ons you haven't mentioned. `addonsConfig` cannot be set on an Autopilot cluster, since Autopilot manages add-ons itself. An unrecognized add-on or option is rejected when you apply the resource, as is an option set on an add-on that isn't enabled.

## Prerequisites

Most GKE add-ons require something of the cluster before they can be enabled — Workload Identity, a minimum version, a particular kind of node. CAPG checks these for you rather than letting GKE reject the cluster later:

- Requirements it can judge from the resource itself, such as Workload Identity or the release channel, are **rejected when you apply it**.
- Requirements that depend on the cluster as it actually exists, such as what your node pools run, are checked **during reconciliation**, and an unmet one is reported on the `GCPManagedControlPlane`'s conditions within seconds of applying.

The `sliceControllerConfig` accelerator requirement is the one exception: `GCPManagedMachinePool` has no accelerator field, so CAPG can't check it at either point. Because nothing else will report it either, it's the one requirement you're warned about when you apply.

## Supported add-ons

| Key | Enables | Options | Requires |
|---|---|---|---|
| `dnsCacheConfig` | NodeLocal DNSCache, a DNS cache running on cluster nodes. | — | GKE 1.15 or later; not supported on Windows node pools |
| `gcePersistentDiskCsiDriverConfig` | The Compute Engine persistent disk CSI driver. | — | — |
| `gcpFilestoreCsiDriverConfig` | The Filestore CSI driver. | — | GKE 1.21 or later; not supported on Windows node pools |
| `gkeBackupAgentConfig` | The Backup for GKE agent. | — | Workload Identity; GKE 1.19 or later |
| `configConnectorConfig` | Config Connector, a Kubernetes extension for managing hosted Google Cloud services through the Kubernetes API. | — | Workload Identity; GKE 1.15.11-gke.5 or later |
| `statefulHaConfig` | The Stateful HA add-on. | — | `gcePersistentDiskCsiDriverConfig` enabled as well; GKE 1.28 or later; node pools on the e2, n1, n2 or n2d machine families |
| `gcsFuseCsiDriverConfig` | The Cloud Storage FUSE CSI driver. | — | Workload Identity |
| `parallelstoreCsiDriverConfig` | The Cloud Storage Parallelstore CSI driver. | — | GKE 1.31.1-gke.1729000 or later; amd64 nodes |
| `lustreCsiDriverConfig` | The Managed Lustre CSI driver. | `disableMultiNic` — turns off multi-NIC support, which the driver otherwise enables. | — |
| `rayOperatorConfig` | The Ray Operator, which manages Ray clusters. | `rayClusterLoggingConfig` — collects logs from Ray clusters. `rayClusterMonitoringConfig` — collects metrics from Ray clusters. | — |
| `highScaleCheckpointingConfig` | The High Scale Checkpointing add-on. | — | Workload Identity; `gcsFuseCsiDriverConfig` enabled as well; GKE 1.32.4-gke.1415000 or later |
| `sliceControllerConfig` | The Slice Controller add-on. | — | The `rapid` release channel; node pools with TPU7x accelerators |
| `agentSandboxConfig` | The AgentSandbox add-on. | — | GKE 1.35.2-gke.1269000 or later; a node pool with `spec.nodeSecurity.sandboxType: GVISOR`; the `artifactregistry.googleapis.com` API enabled on the project |
| `nodeReadinessConfig` | The GKE Node Readiness Controller. Only useful alongside `gcePersistentDiskCsiDriverConfig`. | — | — |
| `podSnapshotConfig` | The Pod Snapshots feature. | — | Workload Identity; GKE 1.35.3-gke.1234000 or later; a node pool with `spec.nodeSecurity.sandboxType: GVISOR`; not supported on the e2 machine family |
| `slurmOperatorConfig` | The Slurm Operator, which manages the compute pods for a Slurm cluster. | — | The `rapid` release channel; GKE 1.35.2-gke.1842000 or later |

Where a version floor is given, CAPG checks it against `spec.version` if you've pinned one, and otherwise against the version GKE reports the cluster is running — so a cluster whose version is managed by a release channel is still checked, from its first reconciliation onwards.

## Notes

- Changes to `addonsConfig` are applied as a partial update — enabling or disabling one add-on doesn't touch any other add-on's current state, whether or not it's mentioned in your spec.
- GKE's older add-ons — Cloud Run, HTTP load balancing, horizontal pod autoscaling, the Kubernetes dashboard and network policy — aren't supported here. GKE expresses them as a `disabled` flag rather than an enabled one, and each is either on by default, deprecated, or superseded. Network policy in particular is handled by Dataplane V2, which enforces Kubernetes NetworkPolicy with no add-on at all — see [Network Configuration](./network-config.md).