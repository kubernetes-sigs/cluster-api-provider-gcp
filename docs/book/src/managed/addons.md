# Add-ons

`spec.addonsConfig` on `GCPManagedControlPlane` lets you explicitly enable or disable GKE add-ons. It's a map, keyed by add-on name, of `true`/`false`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  addonsConfig:
    gcsFuseCsiDriverConfig: true
    dnsCacheConfig: true
```

Leaving an add-on out of the map entirely keeps GKE's own default for it in place — CAPG doesn't impose an opinion on add-ons you haven't mentioned. `addonsConfig` cannot be set on an Autopilot cluster, since Autopilot manages add-ons itself. An unrecognized key is rejected when you apply the resource.

## Supported add-ons

| Key | Enables |
|---|---|
| `dnsCacheConfig` | NodeLocal DNSCache, a DNS cache running on cluster nodes. |
| `gcePersistentDiskCsiDriverConfig` | The Compute Engine persistent disk CSI driver. |
| `gcpFilestoreCsiDriverConfig` | The Filestore CSI driver. |
| `gkeBackupAgentConfig` | The Backup for GKE agent. |
| `configConnectorConfig` | Config Connector, a Kubernetes extension for managing hosted Google Cloud services through the Kubernetes API. |
| `statefulHAConfig` | The Stateful HA add-on. |
| `gcsFuseCsiDriverConfig` | The Cloud Storage FUSE CSI driver. |
| `parallelstoreCsiDriverConfig` | The Cloud Storage Parallelstore CSI driver. |
| `highScaleCheckpointingConfig` | The High Scale Checkpointing add-on. |
| `sliceControllerConfig` | The Slice Controller add-on. |
| `agentSandboxConfig` | The AgentSandbox add-on. |
| `nodeReadinessConfig` | The GKE Node Readiness Controller. |
| `podSnapshotConfig` | The Pod Snapshots feature. |
| `slurmOperatorConfig` | The Slurm Operator, which manages the compute pods for a Slurm cluster. |

A couple of GKE add-ons aren't supported here because they carry more configuration than a plain on/off toggle — notably the Ray Operator (which has its own logging/monitoring sub-configuration) and the Lustre CSI driver (which has a kernel-module-install option). If you need either, please open an issue.

## Notes

- Changes to `addonsConfig` are applied as a partial update — enabling or disabling one add-on doesn't touch any other add-on's current state, whether or not it's mentioned in your spec.
