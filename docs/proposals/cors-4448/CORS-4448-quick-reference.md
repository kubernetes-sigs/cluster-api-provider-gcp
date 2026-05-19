# CORS-4448 Quick Reference

## Summary
Add support for Regional External Load Balancers required by GCD (Google Cloud Distributed/Sovereign Cloud).

## Key API Changes

### 1. New LoadBalancerType Values (`api/v1beta1/types.go`)

```go
// Add these to existing enum:
RegionalExternal = LoadBalancerType("RegionalExternal")
RegionalInternalExternal = LoadBalancerType("RegionalInternalExternal")
```

### 2. New Configuration Field (`api/v1beta1/types.go`)

```go
type LoadBalancerSpec struct {
    // ... existing fields ...
    
    // NEW: Configuration for external load balancer
    ExternalLoadBalancer *LoadBalancer `json:"externalLoadBalancer,omitempty"`
}
```

## Key Software Changes

### 1. Service Interface (`cloud/services/compute/loadbalancers/service.go`)

```go
// ADD: New interface for regional target TCP proxies
type regionaltargettcpproxiesInterface interface {
    Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*compute.TargetTcpProxy, error)
    Insert(ctx context.Context, key *meta.Key, obj *compute.TargetTcpProxy, options ...k8scloud.Option) error
    Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
}

// ADD: New field in Service struct
type Service struct {
    // ... existing fields ...
    regionaltargettcpproxies regionaltargettcpproxiesInterface
}
```

### 2. Core Functions (`cloud/services/compute/loadbalancers/reconcile.go`)

**New Functions to Implement:**

| Function | Purpose |
|----------|---------|
| `isRegionalExternalLoadBalancer()` | Detect if LB type requires regional resources |
| `shouldCreateExternalLoadBalancer()` | Determine if external LB needed |
| `createRegionalExternalLoadBalancer()` | Main orchestration for regional external LB |
| `createOrGetRegionalBackendServiceExternal()` | Regional backend service for external LB |
| **`createOrGetRegionalTargetTCPProxy()`** | **CRITICAL: Regional target TCP proxy** |
| `createOrGetRegionalAddress()` | Regional external address |
| `createOrGetRegionalForwardingRuleWithProxy()` | Regional forwarding rule with proxy target |
| `deleteRegionalExternalLoadBalancer()` | Cleanup orchestration |
| `deleteRegionalAddress()` | Delete regional address |
| `deleteRegionalTargetTCPProxy()` | Delete regional target TCP proxy |

**Functions to Modify:**

| Function | Change |
|----------|--------|
| `Reconcile()` | Add branching logic for regional vs global |
| `Delete()` | Add branching logic for regional vs global deletion |

## Resource Comparison

### Global External LB (Current - `External` type)

| Resource | Key Type |
|----------|----------|
| Health Check | `meta.GlobalKey()` |
| Backend Service | `meta.GlobalKey()` |
| Target TCP Proxy | `meta.GlobalKey()` |
| Address | `meta.GlobalKey()` |
| Forwarding Rule | `meta.GlobalKey()` |

### Regional External LB (New - `RegionalExternal` type)

| Resource | Key Type |
|----------|----------|
| Health Check | `meta.RegionalKey(name, region)` |
| Backend Service | `meta.RegionalKey(name, region)` |
| Target TCP Proxy | `meta.RegionalKey(name, region)` ← **NEW** |
| Address | `meta.RegionalKey(name, region)` |
| Forwarding Rule | `meta.RegionalKey(name, region)` |

## Key Differences from Internal LB

### Regional Internal LB (Passthrough)

- LoadBalancingScheme: `INTERNAL`
- No Target TCP Proxy (direct to backend service)
- Balancing Mode: `CONNECTION`
- Address Type: `INTERNAL`

### Regional External LB (Proxy)

- LoadBalancingScheme: `EXTERNAL`  
- **Uses Target TCP Proxy** ← Key difference
- Balancing Mode: `UTILIZATION` (or `CONNECTION` if dual LB)
- Address Type: `EXTERNAL`

## Usage Examples

### Basic Regional External LB

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  project: my-gcd-project
  region: us-central1
  loadBalancer:
    loadBalancerType: RegionalExternal
```

### Regional External LB with Custom Configuration

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  project: my-gcd-project
  region: us-central1
  loadBalancer:
    loadBalancerType: RegionalExternal
    externalLoadBalancer:
      name: "my-custom-lb"
      ipAddress: "10.1.2.3"
```

### Regional Dual LB (External + Internal)

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  project: my-gcd-project
  region: us-central1
  loadBalancer:
    loadBalancerType: RegionalInternalExternal
    externalLoadBalancer:
      name: "external-api"
    internalLoadBalancer:
      name: "internal-api"
      subnet: "control-plane-subnet"
```

## Implementation Priority

1. **High Priority (Blocking)**
   - [ ] API changes (types.go)
   - [ ] `createOrGetRegionalTargetTCPProxy()` - CRITICAL missing piece
   - [ ] `createRegionalExternalLoadBalancer()` - main orchestration
   - [ ] Update `Reconcile()` branching logic

2. **Medium Priority**
   - [ ] Supporting regional resource functions
   - [ ] Delete logic for cleanup
   - [ ] Unit tests

3. **Lower Priority**
   - [ ] Integration tests
   - [ ] Documentation
   - [ ] CRD generation

## Testing Checklist

- [ ] Unit test: `isRegionalExternalLoadBalancer()`
- [ ] Unit test: Regional health check creation
- [ ] Unit test: Regional backend service creation (EXTERNAL scheme)
- [ ] Unit test: Regional target TCP proxy creation **← CRITICAL**
- [ ] Unit test: Regional address creation (EXTERNAL type)
- [ ] Unit test: Regional forwarding rule with proxy
- [ ] Integration test: Full regional external LB lifecycle
- [ ] Integration test: `RegionalInternalExternal` dual LB
- [ ] Integration test: Resource cleanup

## Common Pitfalls to Avoid

1. **Don't reuse internal LB functions directly** - they use INTERNAL scheme
2. **Regional address needs AddressType: EXTERNAL** - not INTERNAL
3. **Backend service needs LoadBalancingScheme: EXTERNAL** - not INTERNAL  
4. **Forwarding rule points to Target, not BackendService** - different from passthrough LB
5. **Don't forget to add regionaltargettcpproxies to Service struct** - required for regional target TCP proxy

## Files to Modify

```
api/v1beta1/
  ├── types.go                                    # API changes
  └── zz_generated.deepcopy.go                    # Auto-generated

cloud/services/compute/loadbalancers/
  ├── service.go                                   # Add regional target TCP proxy interface
  ├── reconcile.go                                 # Main implementation
  └── reconcile_test.go                            # Unit tests
```

## Verification Commands

After implementation, verify with:

```bash
# Check that regional resources are created
gcloud compute target-tcp-proxies list --regions=us-central1
gcloud compute forwarding-rules list --regions=us-central1
gcloud compute addresses list --regions=us-central1
gcloud compute backend-services list --regions=us-central1
gcloud compute health-checks list --regions=us-central1

# Verify control plane endpoint
kubectl get gcpcluster -o jsonpath='{.status.network}'
```
