# GKE Cluster Upgrades

## Control Plane Upgrade

Upgrading the Kubernetes version of the control plane is supported by the provider. To perform an upgrade you need to update the `controlPlaneVersion` in the spec of the `GCPManagedControlPlane`. Once the version has changed the provider will handle the upgrade for you.

## Node Pool Machine Type

`spec.instanceType` on `GCPManagedMachinePool` is mutable. Changing it does not resize nodes in place: GKE performs the change as a rolling node replacement, so workload disruption is implied, the same as with the control plane upgrade above.

`spec.upgradeSettings` controls how disruptive that rollout is. Both `SURGE` and `BLUE_GREEN` are supported and the 
strategy will default to `SURGE` when unset.

## SURGE Upgrades

An example of configuring a `SURGE` upgrade can be seen below. A `SURGE` upgrade means nodes get updated in a rolling
window so different instances in the node pool will be on different instance types during the rollout. For more
information see [Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/node-pool-upgrade-strategies#surge)

```yaml
spec:
  instanceType: n1-standard-4
  upgradeSettings:
    strategy: SURGE
    maxSurge: 10
    maxUnavailable: 5
```

The default values for `maxSurge` and `maxUnavailable` are 1 and 0 respectively.

## BLUE_GREEN Upgrades

A blue-green Upgrade by contrast brings up an entirely new node pool, the same size as the original but updated, and
migrates workloads to it before destroying the old node pool. This allows workloads to be validated on the new
nodes, while retaining the option to move back if necessary. For more information see
[Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/node-pool-upgrade-strategies#blue-green-upgrade-strategy)

```yaml
spec:
  instanceType: n1-standard-4
  upgradeSettings:
    strategy: BLUE_GREEN
    blueGreenSettings:
      standardRolloutPolicy:
        batchNodeCount: 5
```

There are more options to configure for a blue-green update than a surge update, so below are some things to bear
in mind:

* `strategy: BLUE_GREEN` requires `blueGreenSettings.standardRolloutPolicy` otherwise it will be rejected by the API
  Server
* `standardRolloutPolicy.batchPercentage` is an integer between 1 and 100 inclusive,
  `standardRolloutPolicy.batchNodeCount` must be non-negative. Exactly one of the two must be provided
    * The GKE API takes `batchPercentage` as a float, however this potentially exposes problems of
      [float precision](https://0.30000000000000004.com/) to users. As such we use an integer in the CustomResource and
      convert to a float to send to the API.
* You cannot mix and match, `maxSurge` and `maxUnavailable` are rejected with `strategy: BLUE_GREEN`. In the same way
  `blueGreenSettings` will be rejected if used alongside `strategy: SURGE`
* `standardRolloutPolicy.batchSoakDuration` has a default of 0 and `nodePoolSoakDuration` has a default of 1 hour.
* Due to a new node pool being spun up when using a blue-green strategy costs will increase (2x Compute etc.) for the 
  period where both the blue and green pools exist. Also bear in mind the batch settings control drain **not** the 
  creation of the green pool.
* When a blue-green upgrade is happening, workloads will be rescheduled to the new nodes, however changing only 
  `upgradeSettings` does not change nodes
* The `Autoscaled` rollout policy is not, at present, supported
