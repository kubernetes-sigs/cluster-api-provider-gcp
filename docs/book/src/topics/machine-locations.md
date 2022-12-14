# Machine Locations

This document describes how to configure the location of a CAPG cluster's compute resources. By default, CAPG requires the user to specify a [GCP region](https://cloud.google.com/compute/docs/regions-zones#available) for the cluster's machines by setting the `GCP_REGION` environment variable as outlined in the [CAPI quickstart guide](https://cluster-api.sigs.k8s.io/user/quick-start.html#required-configuration-for-common-providers). The provider then picks a zone to deploy the control plane and worker nodes in and generates the according portions of the cluster's YAML manifests.

It is possible to override this default behaviour and exercise more fine-grained control over machine locations as outlined in the rest of this document.

## Control Plane Machine Location

Before deploying the cluster, add a `failureDomains` field to the `spec` of your `GCPCluster` definition, containing a list of allowed zones:

```diff
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
kind: GCPCluster
metadata:
  name: capi-quickstart
spec:
  network:
    name: default
  project: cyberscan2
  region: europe-west3
+  failureDomains:
+    - europe-west3-b
```

In this example configuration, only a single zone has been added, ensuring the control plane is provisioned in `europe-west3-b`.

## Node Pool Location

Similar to the above, you can override the auto-generated GCP zone for your `MachineDeployment`, by changing the value of the `failureDomain` field at `spec.template.spec.failureDomain`:

```diff
apiVersion: cluster.x-k8s.io/v1alpha4
kind: MachineDeployment
metadata:
  name: capi-quickstart-md-0
spec:
  clusterName: capi-quickstart
  # [...]
  template:
    spec:
      # [...]
      clusterName: capi-quickstart
-      failureDomain: europe-west3-a
+      failureDomain: europe-west3-b
```

When combined like this, the above configuration effectively instructs CAPG to deploy the CAPI equivalent of a [zonal GKE cluster](https://cloud.google.com/kubernetes-engine/docs/concepts/types-of-clusters#availability).