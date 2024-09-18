# Preemptible Virtual Machines

[GCP Preemptible Virtual Machines](https://cloud.google.com/compute/docs/instances/preemptible) allows user to run a VM instance at a much lower price when compared to normal VM instances.

Compute Engine might stop (preempt) these instances if it requires access to those resources for other tasks. Preemptible instances will always stop after 24 hours.

## When do I use Preemptible Virtual Machines?

A Preemptible VM works best for applications or systems that distribute processes across multiple instances in a cluster. While a shutdown would be disruptive for common enterprise applications, such as  databases, it’s hardly noticeable in distributed systems that run across clusters of machines and are designed to tolerate failures.

## How do I use Preemptible Virtual Machines?

To enable a machine to be backed by Preemptible Virtual Machine, add `preemptible` option to `GCPMachineTemplate` and set it to True.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: capg-md-0
spec:
  region: us-west-1
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: STANDARD
      osType: Linux
    vmSize: E2
    preemptible: true
```

## Spot VMs
[Spot VMs are the latest version of preemptible VMs.](https://cloud.google.com/compute/docs/instances/spot)

To use a Spot VM instead of a Preemptible VM, add `provisioningModel` to `GCPMachineTemplate` and set it to `Spot`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: capg-md-0
spec:
  region: us-west-1
  template:
    osDisk:
      diskSizeGB: 30
      managedDisk:
        storageAccountType: STANDARD
      osType: Linux
    vmSize: E2
    provisioningModel: Spot
```

NOTE: specifying `preemptible: true` and `provisioningModel: Spot` is equivalent to only `provisioningModel: Spot`. Spot takes priority. 
