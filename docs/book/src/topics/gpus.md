# GPUs

Add GPUs via the `guestAccelerators` field in `GCPMachineTemplate`.

```
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: mygcpmachinetemplate
  namespace: mynamespace
spec:
  template:
    spec:
      image: projects/myproject/global/images/myimage
      instanceType: n1-standard-2
      guestAccelerators:
      - type: projects/myproject/zones/us-central1-c/acceleratorTypes/nvidia-tesla-t4
        count: 1
```

https://cloud.google.com/compute/docs/gpus

NOTE: Instances with accelerators/GPUs do NOT support live migration. 
Therefore, the `onHostMaintenance` event is always `TERMINATE`.
https://cloud.google.com/compute/docs/instances/setting-vm-host-options
