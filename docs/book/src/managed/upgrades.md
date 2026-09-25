# GKE Cluster Upgrades

## Control Plane Upgrade

Upgrading the Kubernetes version of the control plane is supported by the provider. To perform an upgrade you need to update the `controlPlaneVersion` in the spec of the `GCPManagedControlPlane`. Once the version has changed the provider will handle the upgrade for you.

## Node Pool Machine Type

`spec.instanceType` on `GCPManagedMachinePool` is mutable. Changing it does not resize nodes in place: GKE performs the change as a rolling node replacement, so workload disruption is implied, the same as with the control plane upgrade above.

`spec.upgradeSettings` controls how disruptive that rollout is:

```yaml
spec:
  instanceType: n1-standard-4
  upgradeSettings:
    strategy: SURGE
    maxSurge: 1
    maxUnavailable: 0
```

`maxSurge` and `maxUnavailable` only apply when `strategy` is `SURGE`, which is also GKE's default if `strategy` is left unset. `BLUE_GREEN` is supported as a strategy value, but its nested blue/green rollout configuration (soak durations, batch policy) is not yet exposed here.
