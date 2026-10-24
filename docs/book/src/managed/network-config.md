# Network Configuration

`spec.clusterNetwork` on `GCPManagedControlPlane` also controls two GKE cluster-networking features: the datapath provider (Dataplane V2) and the in-cluster DNS provider.

## Datapath provider (Dataplane V2)

`datapathProvider` selects the implementation of the Kubernetes networking model GKE uses for service resolution and network policy enforcement:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  clusterNetwork:
    datapathProvider: advanced
```

`datapathProvider` accepts:

- **`advanced`** — [GKE Dataplane V2](https://cloud.google.com/kubernetes-engine/docs/how-to/dataplane-v2), an eBPF-based dataplane. This is Google's recommended default for new clusters.
- **`legacy`** — the IPTables-based implementation built on kube-proxy.

Omitting `datapathProvider` leaves GKE's default in place. **This field is immutable once the cluster is created** — GKE does not support switching datapath providers on an existing cluster. It also cannot be set when `enableAutopilot` is `true`: Autopilot clusters always use Dataplane V2.

## Cluster DNS

`dnsConfig` chooses which DNS provider serves in-cluster DNS records:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  clusterNetwork:
    dnsConfig:
      clusterDNS: cloud-dns
      clusterDNSScope: cluster
      clusterDNSDomain: cluster.local
```

- **`clusterDNS`** — `platform` (GKE's default), `cloud-dns`, or `kube-dns`.
- **`clusterDNSScope`** — `cluster` (records resolvable from within the cluster only) or `vpc` (records resolvable from anywhere in the VPC). Only meaningful with `clusterDNS: cloud-dns`.
- **`clusterDNSDomain`** — the suffix used for cluster service records.

`dnsConfig` is mutable — you can change its contents on an existing cluster. However, **once set it cannot be removed entirely**; there's no clean way to revert to "no DNS config" once GKE has one configured.

## Global Access

`controlPlaneGlobalAccess` allows access to the control plane from IP addresses in other Google Cloud regions. This
can be enabled on the `GCPManagedControlPlane` resource as follows:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-control-plane
spec:
  location: us-central1
  project: my-proj
  clusterNetwork:
    privateCluster:
      controlPlaneGlobalAccess: true
```

This is reconciled on create and afterwards by CAPG, and updating is a non-disruptive operation
