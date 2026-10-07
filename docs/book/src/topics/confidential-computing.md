# Confidential Computing

[Confidential VMs](https://cloud.google.com/confidential-computing/confidential-vm/docs/confidential-vm-overview) keep a machine's memory encrypted while it is in use, so the data a workload is actively processing stays encrypted even from the hypervisor.

CAPG can turn this on for both Compute Engine instances and GKE node pools.

## Choosing a technology

The `confidentialCompute` field takes the technology to use, not just an on/off flag:

| Value | Technology | Machine series |
|---|---|---|
| `Disabled` | Confidential computing is off. | — |
| `Enabled` | Whichever technology GCP picks as the default, currently AMD SEV. **The default is subject to change**, so set the technology explicitly if you depend on a particular one. | `n2d`, `c2d`, `c3d` |
| `AMDEncryptedVirtualization` | AMD Secure Encrypted Virtualization (SEV). | `n2d`, `c2d`, `c3d` |
| `AMDEncryptedVirtualizationNestedPaging` | AMD SEV Secure Nested Paging (SEV-SNP). | `n2d` |
| `IntelTrustedDomainExtensions` | Intel Trust Domain Extensions (TDX). | `c3` |

The machine type you choose has to belong to one of the series listed for the technology you pick. CAPG checks this when you apply the resource rather than letting the request fail later, so a mismatch is rejected with the series that would have worked.

## Compute Engine instances

Set `confidentialCompute` on `GCPMachine` or `GCPMachineTemplate`. Confidential VMs cannot be live-migrated, so `onHostMaintenance` must be `Terminate`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: capg-md-0
spec:
  template:
    spec:
      instanceType: n2d-standard-4
      confidentialCompute: AMDEncryptedVirtualization
      onHostMaintenance: Terminate
```

Leaving `onHostMaintenance` unset, or setting it to `Migrate`, is rejected when the resource is applied.

## GKE node pools

For GKE, set `confidentialCompute` under `nodeSecurity` on `GCPManagedMachinePool`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: confidential-nodepool
spec:
  instanceType: n2d-standard-4
  nodeSecurity:
    confidentialCompute: AMDEncryptedVirtualization
```

There is no `onHostMaintenance` to set here — GKE manages host maintenance for its node pools itself.

`nodeSecurity` is **immutable**, and the admission webhook enforces it: once a node pool exists, `nodeSecurity` cannot be changed, added, or removed. To move an existing workload onto confidential nodes, create a new node pool and migrate to it.
