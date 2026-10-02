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
    lustreCsiDriverConfig:
      enabled: true
      options:
        disableMultiNic: true
```

Leaving an add-on out of the map entirely keeps GKE's own default for it in place — CAPG doesn't impose an opinion on add-ons you haven't mentioned. `addonsConfig` cannot be set on an Autopilot cluster, since Autopilot manages add-ons itself. An unrecognized add-on or option is rejected when you apply the resource, as is an option set on an add-on that isn't enabled.

## Prerequisites

Most GKE add-ons require something of the cluster before they can be enabled — Workload Identity, a minimum version, a particular kind of node, a project API. CAPG checks these for you rather than letting GKE reject the cluster later:

- Requirements it can judge from the resource itself, such as Workload Identity or the release channel, are **rejected when you apply it**.
- Requirements that depend on the cluster as it actually exists, such as what your node pools run or which APIs your project has enabled, are checked **during reconciliation**, and an unmet one is reported on the `GCPManagedControlPlane`'s conditions within seconds of applying.

## Supported add-ons

{{#include _addons-table.md}}

Where a version floor is given, CAPG checks it against `spec.version` if you've pinned one, and otherwise against the version GKE reports the cluster is running — so a cluster whose version is managed by a release channel is still checked, from its first reconciliation onwards.

## Notes

- Changes to `addonsConfig` are applied as a partial update — enabling or disabling one add-on doesn't touch any other add-on's current state, whether or not it's mentioned in your spec.
- GKE's older add-ons — Cloud Run, HTTP load balancing, horizontal pod autoscaling, the Kubernetes dashboard and network policy — aren't supported here. GKE expresses them as a `disabled` flag rather than an enabled one, and each is either on by default, deprecated, or superseded. Network policy in particular is handled by Dataplane V2, which enforces Kubernetes NetworkPolicy with no add-on at all — see [Network Configuration](./network-config.md).