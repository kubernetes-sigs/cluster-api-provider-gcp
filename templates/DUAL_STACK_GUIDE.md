# Dual Stack Cluster Configuration Guide

This guide explains how to configure a dual stack (IPv4 + IPv6) GCP cluster with public bootstrap nodes and private worker nodes.

## Overview

The dual stack configuration uses a **two-subnet architecture**:

1. **Public Subnet** - For control plane/bootstrap nodes
   - Has external IPv6 access enabled (`externalIpv6: true`)
   - Uses EXTERNAL IPv6 access type with GUA (Globally Unique Address) ranges
   - Allows instances to receive public IPv4 and IPv6 addresses when `publicIP: true`

2. **Private Subnet** - For worker nodes
   - Has internal IPv6 access (`externalIpv6: false`, the default)
   - Uses INTERNAL IPv6 access type with ULA (Unique Local Address) ranges
   - Required for compatibility with internal load balancers
   - Instances receive private IPv4 and internal IPv6 addresses only

## Key Configuration Fields

### Cluster Network Configuration

When using dual stack, you must configure both IPv4 and IPv6 CIDR blocks for pods and services:

```yaml
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - "192.168.0.0/16"    # IPv4 pod network
      - "fd00:100:64::/48"   # IPv6 pod network
    services:
      cidrBlocks:
      - "10.96.0.0/12"       # IPv4 service network
      - "fd00:100:96::/108"  # IPv6 service network
```

**Important:** When using `addressPreferencePolicy: "IPv6Primary"`, list the IPv6 CIDR blocks **first**:
```yaml
    pods:
      cidrBlocks:
      - "fd00:100:64::/48"   # IPv6 FIRST for IPv6Primary
      - "192.168.0.0/16"     # IPv4 second
```

### GCPCluster Network Configuration

```yaml
spec:
  network:
    stackType: "DualStack"  # Required for dual stack
    addressPreferencePolicy: "IPv4Primary"  # Or "IPv6Primary" - affects control plane endpoint
    autoCreateSubnetworks: false  # Must be false to define custom subnets
    subnets:
      - name: "control-plane-subnet"
        cidrBlock: "10.0.0.0/24"
        externalIpv6: true  # Enable public IPv6
        ipv6CidrRange: ""  # Optional: specify custom IPv6 CIDR (e.g., "fd00:1234:5678::/64")
                           # If omitted, Google will auto-assign from ULA or GUA range
        region: "${GCP_REGION}"
      - name: "worker-subnet"
        cidrBlock: "10.0.1.0/24"
        externalIpv6: false  # Internal IPv6 only (default)
        ipv6CidrRange: ""  # Optional: specify custom ULA range
        region: "${GCP_REGION}"
```

### Machine Configuration

#### Control Plane Nodes (Public Access)
```yaml
spec:
  template:
    spec:
      publicIP: true  # Enable public IPv4 + IPv6 (when subnet has externalIpv6: true)
      subnet: "control-plane-subnet"  # Assign to public subnet
```

#### Worker Nodes (Private Access)
```yaml
spec:
  template:
    spec:
      # publicIP omitted (defaults to false) - private only
      subnet: "worker-subnet"  # Assign to private subnet
```

## Network Access Matrix

| Machine Type | `publicIP` | Subnet `externalIpv6` | IPv4 Address | IPv6 Address | Use Case |
|--------------|------------|----------------------|--------------|--------------|----------|
| Control Plane | `true` | `true` | Public | Public (GUA) | Bootstrap nodes, bastion hosts |
| Control Plane | `true` | `false` | Public | Internal (ULA) | Control plane with NAT |
| Worker | `false` | `false` | Private | Internal (ULA) | Standard worker nodes |
| Worker | `false` | `true` | Private | Internal (ULA)* | Not recommended |

*Note: When `publicIP: false`, instances cannot receive public IPv6 addresses even on external IPv6 subnets.

## Important Constraints

1. **Internal Load Balancers** require subnets with `externalIpv6: false` (INTERNAL IPv6 with ULA)
2. **External IPv6 subnets** cannot be used with internal load balancers
3. **Public IPv6 addresses** are only assigned when:
   - Machine has `publicIP: true` **AND**
   - Subnet has `externalIpv6: true`

## AddressPreferencePolicy

The `addressPreferencePolicy` determines which IP address family is used for the Kubernetes API server endpoint:

- **`IPv4Primary`** (default): The control plane endpoint uses the IPv4 address
  - `spec.controlPlaneEndpoint.host` will be the IPv4 address
  - Both IPv4 and IPv6 load balancers are created
  - Clients can access the API server via either protocol

- **`IPv6Primary`**: The control plane endpoint uses the IPv6 address
  - `spec.controlPlaneEndpoint.host` will be the IPv6 address
  - Both IPv4 and IPv6 load balancers are created
  - Clients must support IPv6 to access the API server endpoint

```yaml
# Example: IPv6-first cluster
network:
  stackType: "DualStack"
  addressPreferencePolicy: "IPv6Primary"
```

**Note:** Both load balancers (IPv4 and IPv6) are always created in dual stack mode. The preference policy only affects which address is published as the primary control plane endpoint.

## Load Balancers in Dual Stack Mode

When `stackType: "DualStack"` is configured:

1. **Two external load balancers are created**:
   - IPv4 forwarding rule: `${CLUSTER_NAME}-apiserver`
   - IPv6 forwarding rule: `${CLUSTER_NAME}-apiserver-ipv6`

2. **Two IP addresses are allocated**:
   - `status.network.apiServerAddress`: IPv4 address
   - `status.network.apiServerIPv6Address`: IPv6 address

3. **Both load balancers route to the same backend**:
   - Same instance groups
   - Same health checks
   - Same backend service

## IPv6 Firewall Rules

Dual stack clusters automatically create additional firewall rules for IPv6 traffic:

### Automatic IPv6 Rules Created

1. **Health Check Rule** (`allow-${CLUSTER_NAME}-healthchecks-ipv6`):
   - Allows health checks from Google Cloud Load Balancer IPv6 ranges
   - Source ranges:
     - `2600:2d00:1:b029::/64`
     - `2600:2d00:1:1::/64`
   - Target: Control plane nodes (tagged with `${CLUSTER_NAME}-control-plane`)
   - Protocol: TCP port 6443

### Custom IPv6 Firewall Rules

You can define custom IPv6 firewall rules in the `GCPCluster` spec:

```yaml
spec:
  network:
    firewall:
      firewallRules:
        - name: "allow-ipv6-ssh"
          allowed:
            - IPProtocol: "TCP"
              ports: ["22"]
          sourceRanges:
            - "2001:db8::/32"  # IPv6 CIDR
          targetTags:
            - "ssh-access"
```

## IPv6 CIDR Range Configuration

You have two options for IPv6 CIDR assignment:

### Option 1: Auto-Assignment (Recommended)

Omit the `ipv6CidrRange` field and let Google Cloud assign the range:

```yaml
subnets:
  - name: "my-subnet"
    cidrBlock: "10.0.0.0/24"
    externalIpv6: false  # Google assigns ULA range (fd00::/8)
```

### Option 2: Manual Assignment

Specify a custom IPv6 CIDR range:

```yaml
subnets:
  - name: "my-subnet"
    cidrBlock: "10.0.0.0/24"
    ipv6CidrRange: "fd00:1234:5678::/64"  # Must be /64 or larger
    externalIpv6: false
```

**Requirements:**
- Range must be `/64` or larger
- For INTERNAL IPv6 (`externalIpv6: false`): must be from ULA range (`fd00::/8`)
- For EXTERNAL IPv6 (`externalIpv6: true`): Google assigns from GUA range
- Ranges must be unique across all subnets in the VPC

## IPv6 Access Types Explained

### INTERNAL IPv6 Access (ULA)

**When to use:** Private subnets, internal load balancers, most cluster nodes

```yaml
externalIpv6: false  # Default
```

**Characteristics:**
- Uses Unique Local Address (ULA) ranges (`fd00::/8`)
- Addresses are NOT routable on the internet
- Compatible with internal load balancers
- Instances can access Google services via IPv6
- Cloud NAT not required for IPv6 (unlike IPv4)

**Use cases:**
- Worker nodes
- Private control planes
- Internal services

### EXTERNAL IPv6 Access (GUA)

**When to use:** Public-facing instances that need internet accessibility

```yaml
externalIpv6: true
```

**Characteristics:**
- Uses Globally Unique Address (GUA) ranges
- Addresses are routable on the internet
- **NOT compatible** with internal load balancers
- Instances with `publicIP: true` receive public IPv6 addresses
- No NAT required - direct internet access

**Use cases:**
- Bootstrap/bastion nodes
- Public-facing control planes
- Edge/ingress nodes

## Architecture Patterns

### Pattern 1: Public Control Plane + Private Workers (Recommended)

Best for most production workloads where control plane needs external access but workers should be private.

```yaml
subnets:
  - name: "control-plane-subnet"
    cidrBlock: "10.0.0.0/24"
    externalIpv6: true   # Public IPv6 for bootstrap
  - name: "worker-subnet"
    cidrBlock: "10.0.1.0/24"
    externalIpv6: false  # Private IPv6 for security

# Control plane machines
publicIP: true
subnet: "control-plane-subnet"

# Worker machines
publicIP: false
subnet: "worker-subnet"
```

**Benefits:**
- Bootstrap nodes accessible via public IPv4 and IPv6
- Workers remain secure on private network
- Internal load balancers work correctly with worker subnet
- Defense in depth security model

### Pattern 2: All Private (Maximum Security)

Best for highly secure environments with VPN/Cloud Interconnect access.

```yaml
subnets:
  - name: "private-subnet"
    cidrBlock: "10.0.0.0/23"
    externalIpv6: false  # All nodes use internal IPv6

# All machines
publicIP: false
subnet: "private-subnet"
```

**Benefits:**
- No public IP exposure
- Reduced attack surface
- Requires VPN/Interconnect for cluster access

### Pattern 3: All Public (Development/Testing)

Best for development clusters that need easy external access.

```yaml
subnets:
  - name: "public-subnet"
    cidrBlock: "10.0.0.0/23"
    externalIpv6: true  # All nodes can get public IPv6

# All machines
publicIP: true
subnet: "public-subnet"
```

**Warning:** Not recommended for production. Cannot use internal load balancers.

## Using the Template

### Prerequisites
Set the following environment variables:
```bash
export CLUSTER_NAME="my-dual-stack-cluster"
export GCP_PROJECT="my-gcp-project"
export GCP_REGION="us-central1"
export GCP_NETWORK_NAME="my-network"
export CONTROL_PLANE_MACHINE_COUNT=3
export WORKER_MACHINE_COUNT=3
export GCP_CONTROL_PLANE_MACHINE_TYPE="n1-standard-2"
export GCP_NODE_MACHINE_TYPE="n1-standard-2"
export KUBERNETES_VERSION="v1.28.0"
export IMAGE_ID="projects/my-project/global/images/family/capi-ubuntu-2004-k8s-v1-28"
```

### Create the Cluster
```bash
clusterctl generate cluster my-cluster \
  --from templates/cluster-template-dual-stack.yaml \
  | kubectl apply -f -
```

## Troubleshooting

### Control plane nodes not getting IPv6 addresses
**Symptoms:** Nodes only have IPv4 addresses, no IPv6

**Solutions:**
- Verify subnet has `externalIpv6: true` in the GCPCluster spec
- Verify machine template has `publicIP: true`
- Check GCP console for subnet IPv6 access type (should be "EXTERNAL")
- Confirm `stackType: "DualStack"` is set on the network
- Check that the network has `enableUlaInternalIpv6: true` (auto-set for dual stack)

### Internal load balancer creation fails
**Symptoms:** Error creating internal load balancer or forwarding rule

**Solutions:**
- Ensure the subnet used by the load balancer has `externalIpv6: false`
- Check that subnet IPv6 access type is "INTERNAL" in GCP console
- Verify `loadBalancerType: Internal` is not used with external IPv6 subnets
- Check that you're not mixing internal LB with external IPv6 subnets

### IPv6 connectivity issues
**Symptoms:** Pods cannot reach IPv6 endpoints

**Solutions:**
- Verify CNI plugin supports dual stack (Calico, Cilium, etc.)
- Check that both IPv4 and IPv6 pod CIDRs are configured in `clusterNetwork.pods.cidrBlocks`
- Ensure firewall rules allow IPv6 traffic
- Verify nodes have IPv6 addresses: `kubectl get nodes -o wide`
- Check pod IPv6 assignments: `kubectl get pods -o wide`

### Control plane endpoint uses wrong IP family
**Symptoms:** Endpoint is IPv4 but you want IPv6 (or vice versa)

**Solutions:**
- Set `addressPreferencePolicy: "IPv6Primary"` for IPv6 endpoint
- Set `addressPreferencePolicy: "IPv4Primary"` (default) for IPv4 endpoint
- **Important:** When using IPv6Primary, ensure client tools support IPv6
- Verify both load balancers are created in GCP console

### IPv6 CIDR conflicts
**Symptoms:** Subnet creation fails with CIDR overlap error

**Solutions:**
- Ensure `ipv6CidrRange` values are unique across all subnets
- Use auto-assignment by omitting `ipv6CidrRange` (recommended)
- For ULA ranges, use different /64 blocks from `fd00::/8`
- Check existing VPC subnets for conflicts

### Worker nodes cannot communicate
**Symptoms:** Nodes or pods cannot reach each other or external services

**Solutions:**
- Ensure Cloud NAT is configured for IPv4 egress (IPv6 doesn't need NAT)
- Verify firewall rules allow traffic between subnets for both IPv4 and IPv6
- Check that all subnets are in the same VPC network
- Verify default firewall rules are not disabled: check `firewall.defaultRulesManagement: "Managed"`
- Ensure the `allow-${CLUSTER_NAME}-healthchecks-ipv6` firewall rule exists

### Bootstrap process fails
**Symptoms:** Control plane nodes fail to initialize

**Solutions:**
- Verify control plane nodes can reach the internet for package downloads
- Check that `publicIP: true` is set for bootstrap nodes
- Ensure external IPv6 subnet is configured if relying on IPv6 connectivity
- Verify firewall rules allow egress traffic
- Check Cloud NAT configuration for IPv4 egress if using private subnet

## Common Gotchas and Best Practices

### ✅ DO

1. **Use auto-assignment for IPv6 CIDRs** - Let Google manage the addressing
2. **Set `autoCreateSubnetworks: false`** - Required for custom subnet configuration
3. **Use separate subnets** - Public subnet for bootstrap, private for workers
4. **Test with IPv4Primary first** - Easier to debug, then switch to IPv6Primary if needed
5. **Enable flow logs during development** - Set `enableFlowLogs: true` on subnets
6. **Use CNI with dual stack support** - Calico, Cilium, or GCP's own CNI

### ❌ DON'T

1. **Don't mix internal LB with external IPv6 subnets** - They're incompatible
2. **Don't manually manage firewall rules** - Use `defaultRulesManagement: "Managed"`
3. **Don't forget cluster network CIDRs** - Must configure both IPv4 and IPv6 ranges
4. **Don't use /128 or small IPv6 ranges** - Use /64 or larger for subnets
5. **Don't expect IPv6-only** - GCP dual stack always includes IPv4
6. **Don't reuse IPv6 CIDRs** - Each subnet needs a unique range

### Performance Considerations

- **IPv6 adds minimal overhead** - Modern networks handle it efficiently
- **No NAT for IPv6** - Direct routing is faster than IPv4 NAT
- **Health check latency** - Same for IPv4 and IPv6
- **Load balancer costs** - Two load balancers (IPv4 + IPv6) incur additional costs

## Migration from IPv4-Only to Dual Stack

**Warning:** Migrating an existing IPv4-only cluster to dual stack requires recreating the cluster. In-place migration is not supported.

### Migration Steps

1. **Create new dual stack cluster** using the template
2. **Migrate workloads** to the new cluster
3. **Update DNS** to point to new load balancer IPs
4. **Decommission old cluster** after validation

### What Changes

| Component | IPv4-Only | Dual Stack |
|-----------|-----------|------------|
| Network stack | `IPv4Only` | `DualStack` |
| Subnet IPv6 access | N/A | `INTERNAL` or `EXTERNAL` |
| Pod addresses | IPv4 only | IPv4 + IPv6 |
| Service addresses | IPv4 only | IPv4 + IPv6 |
| Node addresses | IPv4 only | IPv4 + IPv6 |
| Load balancers | 1 (IPv4) | 2 (IPv4 + IPv6) |
| Firewall rules | IPv4 only | IPv4 + IPv6 |
| API endpoint | IPv4 | IPv4 or IPv6 (based on preference) |

## Advanced Configuration

### Using Custom IPv6 CIDR Ranges

If you need specific IPv6 addressing:

```yaml
subnets:
  - name: "custom-subnet"
    cidrBlock: "10.0.0.0/24"
    ipv6CidrRange: "fd00:1234:5678:abcd::/64"  # Custom ULA range
    externalIpv6: false
```

**Best practices:**
- Use sequential ranges for easier management (e.g., `fd00:1234:5678:1::/64`, `fd00:1234:5678:2::/64`)
- Document your IPv6 addressing scheme
- Leave room for future expansion

### Multiple Regions

For multi-region clusters, use different IPv6 ranges per region:

```yaml
subnets:
  # us-central1
  - name: "us-central1-subnet"
    region: "us-central1"
    cidrBlock: "10.0.0.0/24"
    ipv6CidrRange: "fd00:1234:5678:100::/64"
    
  # us-east1
  - name: "us-east1-subnet"
    region: "us-east1"
    cidrBlock: "10.1.0.0/24"
    ipv6CidrRange: "fd00:1234:5678:200::/64"
```

## Feature Compatibility Matrix

| Feature | IPv4Only | DualStack + IPv4Primary | DualStack + IPv6Primary |
|---------|----------|------------------------|------------------------|
| External Load Balancer | ✅ | ✅ | ✅ |
| Internal Load Balancer | ✅ | ✅ (requires INTERNAL IPv6) | ✅ (requires INTERNAL IPv6) |
| Cloud NAT | ✅ (required) | ✅ (IPv4 only) | ✅ (IPv4 only) |
| Private Google Access | ✅ | ✅ | ✅ |
| Shared VPC | ✅ | ✅ | ✅ |
| Custom Firewall Rules | ✅ | ✅ (both families) | ✅ (both families) |
| Public Bootstrap Nodes | ✅ | ✅ (both families) | ✅ (both families) |

## References

### Documentation
- [GCP Dual Stack Documentation](https://cloud.google.com/vpc/docs/ipv6)
- [GCP IPv6 Subnet Configuration](https://cloud.google.com/vpc/docs/subnets#ipv6-access-type)
- [GCP Load Balancer Firewall Rules](https://cloud.google.com/load-balancing/docs/firewall-rules)
- [Cluster API Provider GCP](https://github.com/kubernetes-sigs/cluster-api-provider-gcp)
- [Kubernetes Dual Stack Networking](https://kubernetes.io/docs/concepts/services-networking/dual-stack/)

### Relevant Code
- Network interface configuration: `cloud/scope/machine.go:329-382`
- Subnet specification: `cloud/scope/cluster.go:296-347`
- IPv6 firewall rules: `cloud/scope/cluster.go:361-388`
- Load balancer setup: `cloud/services/compute/loadbalancers/reconcile.go`
- API types: `api/v1beta1/types.go:506-523`

### Related Features
- `externalIpv6` field (commit 672f6ee7): Enables public IPv6 on bootstrap nodes
- `ipv6CidrRange` field (commit 35b4b8f9): Custom IPv6 CIDR assignment
- `addressPreferencePolicy` field: Control plane endpoint IP family selection
- Automatic IPv6 firewall rules: Health checks for dual stack load balancers
