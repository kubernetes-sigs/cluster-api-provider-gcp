# Load Balancers

The Cluster API Provider GCP (CAPG) supports several types of load balancers for the Kubernetes API Server. By default, a Global External Proxy Load Balancer is created.

You can configure the load balancer type in the `GCPCluster` specification:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  loadBalancer:
    loadBalancerType: External # Default
```

## Supported Load Balancer Types

| Type | Description |
|------|-------------|
| `External` | (Default) Creates a Global External Proxy Load Balancer to manage traffic to backends in multiple regions. |
| `Internal` | Creates a Regional Internal Passthrough Load Balancer to manage traffic to backends in the configured region. |
| `InternalExternal` | Creates both a Global External Proxy Load Balancer and a Regional Internal Passthrough Load Balancer. |
| `RegionalExternal` | Creates a Regional External Load Balancer to manage traffic to backends in the configured region. Requires a proxy-only subnet. |
| `RegionalInternalExternal` | Creates both a Regional External Load Balancer and a Regional Internal Passthrough Load Balancer. Requires a proxy-only subnet. |

## Proxy-only Subnet

When using `RegionalExternal` or `RegionalInternalExternal` load balancer types, you must provide a proxy-only subnet in your VPC network. This subnet is used by Google Cloud to run proxies on your behalf.

You can configure a proxy-only subnet in the `GCPCluster` specification:

```yaml
spec:
  network:
    subnets:
    - name: proxy-only-subnet
      cidrBlock: 10.0.0.0/24
      purpose: REGIONAL_MANAGED_PROXY
      role: ACTIVE
```

## Internal Load Balancer Configuration

When using `Internal`, `InternalExternal`, or `RegionalInternalExternal` load balancer types, you can further configure the internal load balancer:

```yaml
spec:
  loadBalancer:
    loadBalancerType: Internal
    internalLoadBalancer:
      name: custom-internal-lb-name
      subnet: custom-subnet-name
      internalAccess: Regional # Or Global
      ipAddress: 10.0.0.10
```

### Internal Access

The `internalAccess` field determines whether the internal load balancer allows global access or restricts traffic to clients within the same region:

- `Regional` (Default): Only clients in the same region as the load balancer can access it.
- `Global`: Clients from any region can access the load balancer.
