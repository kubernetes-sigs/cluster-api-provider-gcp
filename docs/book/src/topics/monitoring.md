# Monitoring

Configure GKE's monitoring feature via the `monitoringConfig` field on `GCPManagedControlPlane`.

GKE enables [Google Cloud Managed Service for Prometheus](https://cloud.google.com/stackdriver/docs/managed-prometheus) by default on new clusters. You can disable (or explicitly enable) it via `monitoringConfig.managedPrometheusConfig`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: mygcpmanagedcontrolplane
spec:
  monitoringConfig:
    managedPrometheusConfig:
      enabled: false
```

Setting `enabled: false` disables Managed Service for Prometheus on the cluster. Setting it to `true` (or omitting `spec.monitoringConfig` entirely) leaves GKE's default behavior in place.

This field is mutable and can be changed on an existing cluster; other monitoring settings GKE manages for the cluster are left untouched.
