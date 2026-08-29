# Maintenance Policy

GKE periodically performs automatic maintenance on a cluster's control plane and nodes (version upgrades, security patches, etc). By default this can happen at any time. `spec.maintenancePolicy` on `GCPManagedControlPlane` lets you control when that maintenance is allowed to happen, and put temporary holds on it.

## Maintenance windows

You can restrict maintenance to a recurring window in one of two ways — only one of `dailyMaintenanceWindow` or `recurringMaintenanceWindow` may be set on a given policy; setting both is rejected by the API.

A daily window repeats every day at the same time:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  maintenancePolicy:
    dailyMaintenanceWindow:
      startTime: "03:00"
```

A recurring window follows an [RRULE](https://tools.ietf.org/html/rfc5545#section-3.8.5.3), so you can express things like "every Saturday and Sunday". The RRULE is validated when you apply the resource, so a malformed rule is rejected immediately rather than surfacing later as a reconcile error:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  maintenancePolicy:
    recurringMaintenanceWindow:
      window:
        startTime: "2024-01-06T00:00:00Z"
        endTime: "2024-01-08T00:00:00Z"
      recurrence: "FREQ=WEEKLY;BYDAY=SA,SU"
```

If neither field is set, GKE is free to perform maintenance at any time.

## Maintenance exclusions

`maintenanceExclusions` lets you block maintenance during specific windows regardless of the regular schedule above — useful around a product launch or a change freeze. Each entry is keyed by an arbitrary name and defines a start time, end time, and how much maintenance it blocks via `maintenanceExclusionOption`:

- `no-upgrades` (the default if left unset): blocks all upgrades, control plane and nodes alike.
- `no-minor-upgrades`: blocks minor version upgrades; patches are still allowed.
- `no-minor-or-node-upgrades`: blocks minor version upgrades and all node pool upgrades; only control plane patches are allowed.

At most 3 exclusions may use (or default to) `no-upgrades` at a time:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  maintenancePolicy:
    maintenanceExclusions:
      product-launch:
        startTime: "2024-03-01T00:00:00Z"
        endTime: "2024-03-15T00:00:00Z"
        maintenanceExclusionOption: no-upgrades
```

## Disruption budget

`disruptionBudget` sets a minimum interval GKE must wait between automatic control plane version upgrades, independent of the maintenance window/exclusion configuration above:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: my-cluster
spec:
  maintenancePolicy:
    disruptionBudget:
      minorVersionDisruptionInterval: 168h # 7 days
      patchVersionDisruptionInterval: 24h
```

## Notes

- Updates to `maintenancePolicy` are applied through GKE's dedicated maintenance-policy API rather than the general cluster update path, so they land independently of other pending spec changes — you don't need to wait for an unrelated in-flight update to finish before a maintenance policy change takes effect.
- `maintenancePolicy` is only supported on the control plane; it isn't a per-node-pool setting.
