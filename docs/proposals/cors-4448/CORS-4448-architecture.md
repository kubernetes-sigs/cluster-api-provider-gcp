# CORS-4448 Architecture Diagrams

## Current Architecture: Global External Load Balancer

```
┌─────────────────────────────────────────────────────────────────┐
│                        PUBLIC INTERNET                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ HTTPS/TCP 6443
                         │
                         ▼
        ┌────────────────────────────────────┐
        │   Global Forwarding Rule           │  ◄── meta.GlobalKey()
        │   (global resource)                │
        └────────────┬───────────────────────┘
                     │
                     │ Points to Global Target TCP Proxy
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Global Target TCP Proxy          │  ◄── meta.GlobalKey()
        │   (global resource)                │
        └────────────┬───────────────────────┘
                     │
                     │ Points to Global Backend Service
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Global Backend Service           │  ◄── meta.GlobalKey()
        │   (global resource)                │
        │   - LoadBalancingScheme: EXTERNAL  │
        │   - BalancingMode: UTILIZATION     │
        └────────────┬───────────────────────┘
                     │
                     │ Uses Global Health Check
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Global Health Check              │  ◄── meta.GlobalKey()
        │   (global resource)                │
        │   - TCP 6443                       │
        └────────────┬───────────────────────┘
                     │
                     │ Checks backends in multiple zones
                     │
        ┌────────────┴────────────┬──────────────────┐
        ▼                         ▼                  ▼
┌───────────────┐        ┌───────────────┐  ┌───────────────┐
│ Instance Group│        │ Instance Group│  │ Instance Group│
│  (zone-a)     │        │  (zone-b)     │  │  (zone-c)     │
│               │        │               │  │               │
│ ┌───────────┐ │        │ ┌───────────┐ │  │ ┌───────────┐ │
│ │ CP Node 1 │ │        │ │ CP Node 2 │ │  │ │ CP Node 3 │ │
│ └───────────┘ │        │ └───────────┘ │  │ └───────────┘ │
└───────────────┘        └───────────────┘  └───────────────┘

Global Address ────────► External IP: 1.2.3.4  ◄── meta.GlobalKey()
```

**Problem for GCD:** All resources use `meta.GlobalKey()` which creates global resources. 
GCD (Sovereign Cloud) only supports regional resources.

---

## New Architecture: Regional External Load Balancer

```
┌─────────────────────────────────────────────────────────────────┐
│                        PUBLIC INTERNET                           │
│                    (or GCD Network Boundary)                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ HTTPS/TCP 6443
                         │
                         ▼
        ┌────────────────────────────────────┐
        │   Regional Forwarding Rule         │  ◄── meta.RegionalKey(name, region)
        │   (us-central1)                    │
        │   - LoadBalancingScheme: EXTERNAL  │
        └────────────┬───────────────────────┘
                     │
                     │ Points to Regional Target TCP Proxy
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Regional Target TCP Proxy        │  ◄── meta.RegionalKey(name, region)
        │   (us-central1)                    │      ★ CRITICAL NEW RESOURCE
        │   *** NEW RESOURCE TYPE ***        │
        └────────────┬───────────────────────┘
                     │
                     │ Points to Regional Backend Service
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Regional Backend Service         │  ◄── meta.RegionalKey(name, region)
        │   (us-central1)                    │
        │   - LoadBalancingScheme: EXTERNAL  │  ← Different from Internal LB
        │   - BalancingMode: UTILIZATION     │
        └────────────┬───────────────────────┘
                     │
                     │ Uses Regional Health Check
                     │
                     ▼
        ┌────────────────────────────────────┐
        │   Regional Health Check            │  ◄── meta.RegionalKey(name, region)
        │   (us-central1)                    │
        │   - TCP 6443                       │
        └────────────┬───────────────────────┘
                     │
                     │ Checks backends in zones within region
                     │
        ┌────────────┴────────────┬──────────────────┐
        ▼                         ▼                  ▼
┌───────────────┐        ┌───────────────┐  ┌───────────────┐
│ Instance Group│        │ Instance Group│  │ Instance Group│
│ (us-central1-a│        │(us-central1-b)│  │(us-central1-c)│
│               │        │               │  │               │
│ ┌───────────┐ │        │ ┌───────────┐ │  │ ┌───────────┐ │
│ │ CP Node 1 │ │        │ │ CP Node 2 │ │  │ │ CP Node 3 │ │
│ └───────────┘ │        │ └───────────┘ │  │ └───────────┘ │
└───────────────┘        └───────────────┘  └───────────────┘

Regional Address ──────► External IP: 10.1.2.3  ◄── meta.RegionalKey(name, region)
(AddressType: EXTERNAL)                            (Regional external address)
```

**Solution for GCD:** All resources use `meta.RegionalKey(name, region)` which creates 
regional resources compatible with GCD.

---

## Comparison: Regional External vs Regional Internal LB

### Regional External Load Balancer (NEW - for GCD)

```
Internet/GCD Boundary
        │
        ▼
Regional Forwarding Rule
   (LoadBalancingScheme: EXTERNAL)
        │
        ▼
Regional Target TCP Proxy ◄────── PROXY LOAD BALANCER
        │
        ▼
Regional Backend Service
   (LoadBalancingScheme: EXTERNAL)
   (BalancingMode: UTILIZATION)
        │
        ▼
Regional Health Check
        │
        ▼
Instance Groups (zones in region)
```

### Regional Internal Load Balancer (EXISTING)

```
Internal Network Only
        │
        ▼
Regional Forwarding Rule
   (LoadBalancingScheme: INTERNAL)
        │
        │ NO Target TCP Proxy ◄────── PASSTHROUGH LOAD BALANCER
        │
        ▼
Regional Backend Service
   (LoadBalancingScheme: INTERNAL)
   (BalancingMode: CONNECTION)
   (Network: specified)
        │
        ▼
Regional Health Check
        │
        ▼
Instance Groups (zones in region)
```

**Key Differences:**
1. **Target TCP Proxy:** External uses it (proxy LB), Internal doesn't (passthrough LB)
2. **LoadBalancingScheme:** EXTERNAL vs INTERNAL
3. **BalancingMode:** UTILIZATION vs CONNECTION
4. **Network Field:** External doesn't need it, Internal requires it
5. **Address Type:** EXTERNAL vs INTERNAL

---

## Load Balancer Type Decision Tree

```
Start: What load balancer type do I need?
│
├─ Need multi-region support?
│  └─ YES ──► Use: External (Global External Proxy LB)
│             ├─ meta.GlobalKey() for all resources
│             └─ Works in: Standard GCP
│
├─ Deploying to GCD (Sovereign Cloud)?
│  │
│  ├─ Need external access?
│  │  └─ YES ──► Use: RegionalExternal (Regional External Proxy LB)
│  │             ├─ meta.RegionalKey() for all resources
│  │             ├─ Requires: Regional Target TCP Proxy
│  │             └─ Works in: GCD
│  │
│  ├─ Need both external AND internal access?
│  │  └─ YES ──► Use: RegionalInternalExternal
│  │             ├─ Creates: Regional External + Regional Internal
│  │             ├─ External: meta.RegionalKey()
│  │             ├─ Internal: meta.RegionalKey()
│  │             └─ Works in: GCD
│  │
│  └─ Need only internal access?
│     └─ YES ──► Use: Internal (Regional Internal Passthrough LB)
│                ├─ meta.RegionalKey() for all resources
│                ├─ No Target TCP Proxy
│                └─ Works in: GCD and Standard GCP
│
└─ Standard GCP + need both external and internal?
   └─ YES ──► Use: InternalExternal
              ├─ External: meta.GlobalKey()
              ├─ Internal: meta.RegionalKey()
              └─ Works in: Standard GCP
```

---

## Resource Lifecycle: Regional External LB

### Creation Order (createRegionalExternalLoadBalancer)

```
1. Regional Health Check
   ↓
2. Regional Backend Service ────► Links to: Health Check
   ↓
3. Regional Target TCP Proxy ───► Links to: Backend Service  ★ CRITICAL
   ↓
4. Regional Address
   ↓
5. Regional Forwarding Rule ────► Links to: Target TCP Proxy, Address
```

### Deletion Order (deleteRegionalExternalLoadBalancer)

```
1. Regional Forwarding Rule
   ↓
2. Regional Address
   ↓
3. Regional Target TCP Proxy  ★ CRITICAL
   ↓
4. Regional Backend Service
   ↓
5. Regional Health Check
```

**Note:** Deletion order is reverse of creation to respect resource dependencies.

---

## Code Flow Diagram

### Reconcile() Decision Flow

```
Reconcile()
│
├─ Create Instance Groups (zonal)
│
├─ Get LoadBalancerType
│  └─ Default: External
│
├─ Should create external LB?
│  ├─ YES ──► Is regional external?
│  │          ├─ YES ──► createRegionalExternalLoadBalancer()
│  │          │          ├─ Health Check (regional)
│  │          │          ├─ Backend Service (regional, EXTERNAL)
│  │          │          ├─ Target TCP Proxy (regional) ★
│  │          │          ├─ Address (regional, EXTERNAL)
│  │          │          └─ Forwarding Rule (regional, EXTERNAL)
│  │          │
│  │          └─ NO ───► createExternalLoadBalancer()
│  │                     ├─ Health Check (global)
│  │                     ├─ Backend Service (global)
│  │                     ├─ Target TCP Proxy (global)
│  │                     ├─ Address (global)
│  │                     └─ Forwarding Rule (global)
│  │
│  └─ NO ───► Skip external LB
│
└─ Should create internal LB?
   ├─ YES ──► createInternalLoadBalancer()
   │          ├─ Health Check (regional)
   │          ├─ Backend Service (regional, INTERNAL)
   │          ├─ Address (regional, INTERNAL)
   │          └─ Forwarding Rule (regional, INTERNAL)
   │             (No Target TCP Proxy - passthrough)
   │
   └─ NO ───► Skip internal LB
```

### Helper Function: isRegionalExternalLoadBalancer()

```go
func isRegionalExternalLoadBalancer(lbType) bool {
    return lbType == RegionalExternal || 
           lbType == RegionalInternalExternal
}
```

### Helper Function: shouldCreateExternalLoadBalancer()

```go
func shouldCreateExternalLoadBalancer(lbType) bool {
    return lbType == External ||
           lbType == InternalExternal ||
           lbType == RegionalExternal ||
           lbType == RegionalInternalExternal
}
```

---

## Summary of Changes

### Files Modified

| File | Changes |
|------|---------|
| `api/v1beta1/types.go` | Add `RegionalExternal`, `RegionalInternalExternal` enums<br>Add `ExternalLoadBalancer` field to `LoadBalancerSpec` |
| `cloud/services/compute/loadbalancers/service.go` | Add `regionaltargettcpproxiesInterface`<br>Add field to `Service` struct<br>Initialize in `New()` |
| `cloud/services/compute/loadbalancers/reconcile.go` | Add 9 new functions<br>Modify `Reconcile()` and `Delete()` |
| `cloud/services/compute/loadbalancers/reconcile_test.go` | Add unit tests for new functions |

### New Functions (9 total)

1. `isRegionalExternalLoadBalancer()` - Helper
2. `shouldCreateExternalLoadBalancer()` - Helper
3. `createRegionalExternalLoadBalancer()` - Main orchestration
4. `createOrGetRegionalBackendServiceExternal()` - Regional backend service
5. **`createOrGetRegionalTargetTCPProxy()`** - **CRITICAL**
6. `createOrGetRegionalAddress()` - Regional address
7. `createOrGetRegionalForwardingRuleWithProxy()` - Regional forwarding rule
8. `deleteRegionalExternalLoadBalancer()` - Cleanup orchestration
9. `deleteRegionalTargetTCPProxy()` - Delete target TCP proxy

### Modified Functions (2 total)

1. `Reconcile()` - Add branching logic
2. `Delete()` - Add branching logic

---

## Testing Strategy

### Unit Tests

```go
// Test detection helpers
TestIsRegionalExternalLoadBalancer()
TestShouldCreateExternalLoadBalancer()

// Test creation flow
TestCreateRegionalExternalLoadBalancer()
TestCreateOrGetRegionalTargetTCPProxy()
TestCreateOrGetRegionalAddress()
TestCreateOrGetRegionalBackendServiceExternal()
TestCreateOrGetRegionalForwardingRuleWithProxy()

// Test deletion flow
TestDeleteRegionalExternalLoadBalancer()
TestDeleteRegionalTargetTCPProxy()
```

### Integration Tests

```go
// End-to-end tests
TestRegionalExternalLoadBalancerLifecycle()
TestRegionalInternalExternalLoadBalancerLifecycle()
TestMigrationFromExternalToRegionalExternal()  // Should require new cluster
```

### Manual Testing in GCD

```bash
# 1. Create cluster with regional external LB
cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: test-gcd-cluster
spec:
  project: my-gcd-project
  region: us-central1
  loadBalancer:
    loadBalancerType: RegionalExternal
EOF

# 2. Verify regional resources created
gcloud compute target-tcp-proxies list --regions=us-central1
gcloud compute forwarding-rules list --regions=us-central1

# 3. Test control plane endpoint
kubectl get gcpcluster test-gcd-cluster -o jsonpath='{.spec.controlPlaneEndpoint}'

# 4. Delete and verify cleanup
kubectl delete gcpcluster test-gcd-cluster
gcloud compute target-tcp-proxies list --regions=us-central1  # Should be empty
```
