# Autoscaling from Zero

## Overview

CAPG supports autoscaling from zero replicas by populating `Status.Capacity` and `Status.NodeInfo` on `GCPMachineTemplate`. This enables [cluster-autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) to scale NodePools from 0 replicas without requiring existing nodes.

This follows the [CAPI autoscaling-from-zero proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md) and matches the implementations in CAPA (AWS) and CAPZ (Azure).

**Key benefits:**
- Cost optimization by scaling unused node pools to zero
- Efficient resource utilization for dev/test environments  
- Support for batch workloads that scale between job runs

## When do I use autoscaling from zero?

Autoscaling from zero is useful when you want to:
- Reduce costs by scaling NodePools down to zero replicas when not in use
- Let cluster-autoscaler automatically create nodes when workloads need them
- Support dynamic workloads that may need specialized node pools (high-memory, high-CPU) only occasionally

## How It Works

CAPG's GCPMachineTemplate controller automatically populates status fields when a template is created or reconciled:

1. The controller queries the GCP Compute API for machine type specifications
2. It extracts capacity information (CPU cores, memory) from the machine type
3. It determines node architecture (amd64/arm64) from the machine type's CPU platform
4. It queries the GCP Images API to detect the operating system (linux/windows) from image metadata
5. This information is written to `status.capacity` and `status.nodeInfo` fields

The cluster-autoscaler reads these status fields to simulate node capacity for pending pods, enabling scale-from-zero decisions without requiring actual nodes to exist.

The controller respects cluster pause annotations and requires the template to have an owner reference to a Cluster resource.

## Example

After creating a `GCPMachineTemplate`, CAPG automatically populates the status:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: worker-node-pool
spec:
  template:
    spec:
      instanceType: n1-standard-2
      imageFamily: ubuntu-2004-lts
      imageProject: gke-node-images
status:
  capacity:
    cpu: "2"
    memory: 7680Mi
  nodeInfo:
    architecture: amd64
    operatingSystem: linux
```

No manual configuration needed — CAPG queries GCP and populates these values automatically based on the machine type and image specified in `.spec.template.spec`.

## Status Fields

The CAPG controller populates the following fields in GCPMachineTemplate status:

| Field | Description | Example | Source |
|-------|-------------|---------|--------|
| `status.capacity.cpu` | Number of vCPUs | `"2"`, `"4"`, `"96"` | GCP Compute API (MachineTypes) |
| `status.capacity.memory` | Memory size | `"7680Mi"`, `"16Gi"` | GCP Compute API (MachineTypes) |
| `status.nodeInfo.architecture` | CPU architecture | `amd64`, `arm64` | GCP Compute API (CPU platform) |
| `status.nodeInfo.operatingSystem` | OS type | `linux`, `windows` | GCP Images API (image metadata) |

Inspect the status of a GCPMachineTemplate:

```bash
kubectl get gcpmachinetemplate worker-node-pool -o jsonpath='{.status}' | jq
```

Example output:
```json
{
  "capacity": {
    "cpu": "2",
    "memory": "7680Mi"
  },
  "nodeInfo": {
    "architecture": "amd64",
    "operatingSystem": "linux"
  }
}
```

## Using with Cluster Autoscaler

### Prerequisites

Cluster-autoscaler requires:

1. **RBAC permissions** in the management cluster:
   - Read/write access to CAPI resources (`MachineDeployment`, `MachineSet`, `Machine`)
   - Read access to infrastructure templates (`GCPMachineTemplate`) for scale-from-zero
   - See [RBAC setup for scale-from-zero](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#rbac-changes-for-scaling-from-zero)

2. **Workload cluster kubeconfig**:
   - Cluster-autoscaler needs access to both management cluster (for CAPI resources) and workload cluster (for pod scheduling)
   - See [kubeconfig configuration](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md#connecting-cluster-autoscaler-to-cluster-api-management-and-workload-clusters) for setup options

For a complete reference implementation including RBAC manifests and deployment configuration, see [test/e2e/data/cluster-autoscaler/](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/test/e2e/data/cluster-autoscaler).

### Configuration

To enable scale-from-zero:

1. Deploy cluster-autoscaler in your management cluster with `--cloud-provider=clusterapi`
2. Configure autoscaler to target your MachineDeployment (see example below)
3. Set MachineDeployment replicas to 0 (or allow autoscaler to scale down)
4. When pods are unschedulable, autoscaler reads `GCPMachineTemplate.Status.Capacity` and scales up

Example MachineDeployment configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: worker-md-0
  annotations:
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "0"
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "10"
spec:
  clusterName: my-cluster
  replicas: 0
  template:
    spec:
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: GCPMachineTemplate
        name: worker-node-pool
```

The annotations tell cluster-autoscaler the min/max bounds. With `replicas: 0`, the pool starts at zero and autoscaler scales up when needed.

## Supported Machine Types

All GCP machine types are supported — standard, high-memory, high-CPU, ARM (T2A/T2D), and custom machine types. CAPG queries the GCP Compute API for each machine type to get accurate CPU/memory values.

For ARM-based machine types (e.g., `t2a-standard-2`), `Status.NodeInfo.Architecture` is automatically set to `arm64`.

## Supported Operating Systems

CAPG detects the operating system from the image specified in `GCPMachineTemplate.Spec.Template.Spec.Image`:
- **Linux** — Ubuntu, Debian, COS, RHEL, Fedora, Rocky Linux (default)
- **Windows** — Windows Server images

The OS is detected from GCP image metadata and populated in `Status.NodeInfo.OperatingSystem`.

## Related Resources

- [Cluster API Autoscaling](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/autoscaling) - Cluster API autoscaling overview
- [Autoscaling from Zero Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20210310-opt-in-autoscaling-from-zero.md) - CAPI proposal for scale-from-zero
- [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) - Cluster autoscaler documentation
- [Cluster Autoscaler with Cluster API](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/README.md) - Using cluster-autoscaler with CAPI
- [GCP Machine Types](https://cloud.google.com/compute/docs/machine-types) - Available GCP machine types and specifications
