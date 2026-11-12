# NIC (Network Interface Card) Type

CAPG allows you to configure the type of the primary network interface of any instances that are part of a self-managed cluster.

This is achieved by setting `spec.nicType` on `GCPMachine` and `GCPMachinePool`, and then `spec.template.spec.nicType` on `GCPMachineTemplate`. We currently support:

- Infrastructure Data Plane Function - `IDPF` - [Documentation](https://docs.cloud.google.com/compute/docs/networking/using-idpf)
- Google Virtual NIC - `GVNIC` - [Documentation](https://docs.cloud.google.com/compute/docs/networking/using-gvnic)
- VirtIO-Net - `VIRTIO_NET` - [Documentation](https://docs.oasis-open.org/virtio/virtio/v1.2/virtio-v1.2.html)

For example:

```yaml 
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: workers-with-gvnic
spec:
  template:
    spec:
      instanceType: n2-standard-2
      nicType: GVNIC
```

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachinePool
metadata:
  name: pool-with-gvnic
spec:
  instanceType: n2-standard-2
  nicType: GVNIC
```

This setting is immutable for machines and, as CAPG does not replace existing VMs, any updates to VMs will be governed by the instance group's `updatePolicy`. 

This feature should be used with  care as validation of values is only done at the point at which an API request hits GCP, so check
CAPG logs for failures if this feature does not work as intended.
