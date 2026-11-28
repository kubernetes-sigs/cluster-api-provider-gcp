# CPU Platform

CAPG allows you to set the minimum CPU Platform for any instances that are provisioned as part of a self-managed cluster. This is particularly useful if you have workloads that are sensitive to processor family.

This can be set via the `spec.minCpuPlatform` field on `GCPMachine` and `GCPMachinePool` and the `spec.template.spec.minCpuPlatform`
field on `GCPMachineTemplate`.

Examples of this are included below:

```yaml GCPMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata: 
  name: workers-with-min-cpu-platform
spec:
  template:
    spec: 
      instanceType: n2-standard-2
      minCpuPlatform: "Intel Cascade Lake"
```

```yaml GCPMachinePool
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachinePool
metadata: 
  name: pool-with-min-cpu-platform
spec:
  instanceType: n2-standard-2
  minCpuPlatform: "Intel Cascade Lake"
```

This setting is immutable for machines and changes on a pool will only reach new instances. This feature should be used with
care as validation of values is only done at the point at which an API request hits GCP, so check CAPG logs for failures
if this feature does not work as intended.
