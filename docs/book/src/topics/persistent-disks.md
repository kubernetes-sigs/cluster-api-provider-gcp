# Persistent Disks
---

This document describes how persistent disks are to be provisioned and attached to VMs in Google Cloud Platform.

## Storage Disks
---

See [Storage Options](https://cloud.google.com/compute/docs/disks) for more information.

### Disk Types

We can either configure a zonal or regional persistent disk, we can choose following disk types: 
- Standard persistent disks (`pd-standard`)
- Balanced persistent disks (`pd-balanced`)
- SSD persistent disks (`pd-ssd`)
- Extreme persistent disks (`pd-extreme`)

If you create a disk in the Cloud Console, the default disk type is `pd-balanced`. If you create a disk using the gcloud tool the default disk type is `pd-standard`.

## Disk Specification
---

- `DISK_NAME`: the name of the new disk
- `DISK_SIZE`: the size, in gigabytes, of the new disk. Acceptable sizes range, in 1 GB increments, from 10 GB to 65,536 GB inclusive.
- `DISK_TYPE`: ull or partial URL for the type of the persistent disk. Example: `https://www.googleapis.com/compute/v1/projects/PROJECT_ID/zones/ZONE/diskTypes/pd-ssd`

### Creating disk

```sh
gcloud compute disks create *(DISK_NAME)* \
--size *(DISK_SIZE)* \
--zone *(ZONE)*\
--type *(DISK_TYPE)*
```

### Attaching disk to running or stopped VM instance

```sh
gcloud compute instance attach-disk *(INSTANCE_NAME)* \
--disk *(DISK_NAME)*
```

After this, use `gcloud compute disks describe` to see the description of the disks.

After you create and attach the new disk to a VM, you must format and mount the disk, so that the operating system can use the available storage space.

## Restrictions
---
- You can attach up to 127 secondary non-boot zonal persistent disks.
- You can have a total attached capacity of 257 TB per instance.

## EXAMPLE
---
The below example shows how to create and attach a custom disk "my_disk" at vm-instance-1 for every control plane machine, in addition to the etcd disk. NOTE: the same can be applied to the worker machine.

```yaml
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1alpha4
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
    [...]
    diskSetup:
      resources:
        - name: vm-instance-1
          type: compute.v1.instance
          properties:
            zone: us-central1-f
            machineType: n1-standard-2
          disks:
          - deviceName: my_disk
            type: PERSISTENT
            boot: true
            autoDelete: false
          networkInterfaces:
          - network: https://www.googleapis.com/compute/v1/projects/"${GCP_PROJECT}"/global/networks/default
            accessConfigs:
            - name: External NAT
              type: ONE_TO_ONE_NAT
          layout: true
          overwrite: false
        - name: vm-instance-2
          type: compute.v1.instance
          properties:
            zone: us-central1-f
            machineType: n1-standard-2
          disks:
          - deviceName: etcd_disk
          networkInterfaces:
          - network: https://www.googleapis.com/compute/v1/projects/"${GCP_PROJECT}"/global/networks/default
            accessConfigs:
            - name: External NAT
              type: ONE_TO_ONE_NAT
          layout: true
          overwrite: false
      filesystems:
        - label: etcd_disk
          filesystem: ext4
          name: vm-instance-2
        - label: my_disk
          filesystem: ext4
          name: vm-instance-1
    mounts:
      - - LABEL=etcd_disk
        - /var/lib/etcddisk
      - - LABEL=my_disk
        - /var/lib/mydir
---
kind: GCPMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha4
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      [...]
      dataDisks:
        - nameSuffix: etcddisk
          diskSizeGB: 256
        - nameSuffix: my_disk
          diskSizeGB: 128
```
