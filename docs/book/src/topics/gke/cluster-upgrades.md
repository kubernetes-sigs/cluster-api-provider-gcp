# GKE Cluster Upgrades

## Control Plane Upgrade

Upgrading the Kubernetes version of the control plane is supported by the provider. To perform an upgrade you need to update the `controlPlaneVersion` in the spec of the `GCPManagedControlPlane`. Once the version has changed the provider will handle the upgrade for you.
