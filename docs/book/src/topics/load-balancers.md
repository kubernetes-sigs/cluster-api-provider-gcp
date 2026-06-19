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
| `RegionalExternal` | Creates a Regional External Load Balancer to manage traffic to backends in the configured region. Requires a proxy-only subnet and the **RegionalLoadBalancer** feature flag. |
| `RegionalInternalExternal` | Creates both a Regional External Load Balancer and a Regional Internal Passthrough Load Balancer. Requires a proxy-only subnet and the **RegionalLoadBalancer** feature flag. |

## Enabling Regional Load Balancers

The `RegionalExternal` and `RegionalInternalExternal` load balancer types are experimental and disabled by default. They are enabled via the **RegionalLoadBalancer** feature flag, which can be set before running `clusterctl init` by using the **EXP_CAPG_REGIONAL_LOAD_BALANCER** environment variable:

```shell
export EXP_CAPG_REGIONAL_LOAD_BALANCER=true
clusterctl init --infrastructure gcp
```

When the feature flag is disabled, the controller refuses to reconcile clusters that use these load balancer types. Clusters created while the flag was enabled can still be deleted after it is disabled.

## Proxy-only Subnet

When using `RegionalExternal` or `RegionalInternalExternal` load balancer types, you must provide a [proxy-only subnet](https://cloud.google.com/load-balancing/docs/proxy-only-subnets) in your VPC network, in the same region as the cluster. This subnet is used by Google Cloud to run proxies on your behalf.

You can configure a proxy-only subnet in the `GCPCluster` specification:

```yaml
spec:
  region: us-central1
  network:
    subnets:
    - name: proxy-only-subnet
      cidrBlock: 10.0.0.0/24
      purpose: REGIONAL_MANAGED_PROXY
      region: us-central1 # Optional, defaults to the cluster region
```

The subnet is created with the `ACTIVE` role.

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
