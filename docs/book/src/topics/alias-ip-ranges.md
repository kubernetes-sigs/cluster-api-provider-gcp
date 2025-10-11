# Alias IP Ranges

Configure secondary IP ranges for instances via the `aliasIPRanges` field in `GCPMachineTemplate`.

This enables CNI plugins like Cilium to use [Native Routing](https://docs.cilium.io/en/stable/network/concepts/routing/#google-cloud) by allocating pod and service IPs from the alias ranges.

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
      image: projects/myproject/global/images/myimage
      instanceType: n1-standard-2
      aliasIPRanges:
      - ipCidrRange: /24
        subnetworkRangeName: pods
      - ipCidrRange: 10.96.0.0/16
        subnetworkRangeName: services
```

The `ipCidrRange` accepts:
- CIDR notation: `10.0.0.0/24`
- IP address only: `10.0.0.1`
- Netmask only: `/24`

The `subnetworkRangeName` is optional and references a secondary IP range configured on the subnet.

https://cloud.google.com/vpc/docs/alias-ip