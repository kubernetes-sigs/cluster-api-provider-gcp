# GKE Cluster Upgrades

## Control Plane Upgrade

Upgrading the Kubernetes version of the control plane is supported by the provider. To perform an upgrade you need to update the `controlPlaneVersion` in the spec of the `GCPManagedControlPlane`. Once the version has changed the provider will handle the upgrade for you.

## Node Pool Machine Type

`spec.instanceType` on `GCPManagedMachinePool` is mutable. Changing it does not resize nodes in place: GKE performs the change as a rolling node replacement, so workload disruption is implied, the same as with the control plane upgrade above.

`spec.upgradeSettings` controls how disruptive that rollout is, at present only `SURGE` update strategies are supported,
so strategy will also default to `SURGE` when unset:

```yaml
spec:
  instanceType: n1-standard-4
  upgradeSettings:
    strategy: SURGE
    maxSurge: 10
    maxUnavailable: 5
```

The default values for `maxSurge` and `maxUnavailable` are 1 and 0 respectively.
