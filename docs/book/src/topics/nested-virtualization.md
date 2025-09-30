# Nested Virtualization

Enable nested virtualization to run VMs inside GCE instances via the `enableNestedVirtualization` field. This allows running container sandboxes, KVM, QEMU, or other hypervisors inside the instance. Requires Intel Haswell or later CPU platforms.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: mygcpmachinetemplate
  namespace: mynamespace
spec:
  template:
    spec:
      instanceType: n2-standard-8
      enableNestedVirtualization: true
```

https://cloud.google.com/compute/docs/instances/nested-virtualization/overview

NOTE: Nested virtualization must be enabled at instance creation time and cannot be changed after the instance is created.