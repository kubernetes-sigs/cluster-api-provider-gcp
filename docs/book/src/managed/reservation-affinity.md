# Reservation Affinity

`spec.reservationAffinity` on `GCPManagedMachinePool`s (or `GCPManagedMachinePoolTemplate`s) controls the behaviour that GKE takes towards any [compute reservations](https://docs.cloud.google.com/compute/docs/instances/reservations-overview) that you want to use with GKE node pools. For further information about this feature see [Google's Documentation](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/consuming-reservations)

Reservations allow you to reserve compute capacity with a high degree of confidence, so that you can have a higher degree of certainty that your cluster has node capacity when required.

Note: `spec.reservationAffinity` is immutable and can only be set at creation. Therefore if it needs to be changed you must create a new pool to do so.

## Configuring Reservation Affinity

To configure Reservation Affinity, set the `reservationAffinity` key on `GCPManagedMachinePool` as below:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedMachinePool
metadata:
  name: my-nodepool
spec:
  reservationAffinity:
    consumeReservationType: "NO_RESERVATION"
```

There are four values that `consumeReservationType` can be set to, which each induce different behavior:

- `NO_RESERVATION` - Explicitly do not take capacity from any reservations, instead draw it all from the general pool
- `ANY_RESERVATION` - Take capacity from any reservation that is available
- `ANY_RESERVATION_THEN_FAIL` - Take capacity from any reservation but fail if there is insufficient capacity in the reservations
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