# Reservation Affinity

Reservation Affinity (the behaviour that GCP takes towards utilising compute reservations) is configurable via CAPG for managed and self-managed clusters.
Reservations allow you to reserve compute capacity with a high degree of confidence, so that you can have a higher degree of certainty that your cluster has node
capacity when required.

In the case of self-managed clusters this feature can be accessed `spec.reservationAffinity` on `GCPMachine`, `GCPMachineTemplate` or `GCPMachinePool` and more information can be found here [compute reservations](https://docs.cloud.google.com/compute/docs/instances/reservations-overview). In the case of managed clusters this can be used for `GCPManagedMachinePool` or `GCPManagedMachinePoolTemplate`. For further information about this feature see [Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/consuming-reservations)

## Configuring Reservation Affinity

### Self-Managed Clusters

To configure Reservation Affinity for self-managed clusters you can set the appropriate key on any of the listed 
resources, as below:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachineTemplate
metadata:
  name: workers-with-reservations
spec:
  template:
    spec:
      instanceType: n2-standard-2
      reservationAffinity:
        consumeReservationType: "ANY_RESERVATION"
```

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPMachinePool
metadata:
  name: pool-with-reservations
spec:
  instanceType: n2-standard-2
  reservationAffinity:
    consumeReservationType: "ANY_RESERVATION"
```

### Managed Clusters

To configure Reservation Affinity for managed clusters, set the `reservationAffinity` key on `GCPManagedMachinePool` as below:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: my-nodepool
spec:
  reservationAffinity:
    consumeReservationType: "NO_RESERVATION"
```
### Acceptable values for `consumeReservationType`

There are three (four for managed clusters) values that `consumeReservationType` can be set to, which each induce different behavior:

- `NO_RESERVATION` - Explicitly do not take capacity from any reservations, instead draw it all from the general pool
- `ANY_RESERVATION` - Take capacity from any reservation that is available
- `SPECIFIC_RESERVATION` - Take capacity from a specific named reservation (see below)

`SPECIFIC_RESERVATION` is special because it requires two other configuration settings `key` and `values`, as can be seen below

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: my-nodepool
spec:
  reservationAffinity:
    consumeReservationType: "SPECIFIC_RESERVATION"
    key: "compute.googleapis.com/reservation-name"
    values:
      - "my-reservation"
```

`key` is the property and `values` is the set of values that need to match for capacity to be consumed.

`key` must be exactly `compute.googleapis.com/reservation-name` and values must list the names of reservations. If you set these values for any other `consumeReservationType` you will get a validation error.

If using a managed cluster you can also set the value to `ANY_RESERVATION_THEN_FAIL`, this takes capacity from any 
reservation but fails if there is insufficient capacity in the reservations.

## Immutability

`spec.reservationAffinity` is mutable on some resources and immutable on others:

- `GCPMachine` or `GCPMachineTemplate` - Changes are not allowed for existing machines, users should create a new template or rollout replacement machines
- `GCPMachinePool` - Any changes here cause a new instance template to be created. CAPG does not replace existing VMs, so any updates to VMs will be governed by the instance group's `updatePolicy`.
- `GCPManagedMachinePool` or `GCPManagedMachinePoolTemplate` - Field is immutable and any changes require a new pool