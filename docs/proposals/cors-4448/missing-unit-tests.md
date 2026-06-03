# Missing Unit Tests for CORS-4448

## Summary

The implementation added 13 new/modified functions but **0 unit tests**. All new code is currently untested.

## Required Unit Tests

### 1. Helper Functions (2 tests needed)

#### `TestIsRegionalExternalLoadBalancer`
Tests the helper that detects regional external LB types.

**Test Cases:**
- ✅ Returns true for `RegionalExternal`
- ✅ Returns true for `RegionalInternalExternal`
- ✅ Returns false for `External`
- ✅ Returns false for `Internal`
- ✅ Returns false for `InternalExternal`

**Complexity:** Low - Pure function, no mocks needed

#### `TestShouldCreateExternalLoadBalancer`
Tests the helper that determines if external LB should be created.

**Test Cases:**
- ✅ Returns true for `External`
- ✅ Returns true for `InternalExternal`
- ✅ Returns true for `RegionalExternal`
- ✅ Returns true for `RegionalInternalExternal`
- ✅ Returns false for `Internal`

**Complexity:** Low - Pure function, no mocks needed

---

### 2. Creation Functions (5 tests needed)

#### `TestService_createOrGetRegionalBackendServiceExternal`
Tests regional backend service creation for external LB.

**Test Cases:**
- Backend service does not exist (should create)
  - Uses EXTERNAL LoadBalancingScheme (not INTERNAL)
  - Uses RegionalKey
  - Sets Region field
  - No Network field set
- Backend service exists (should get)
- Backend service exists but needs update (should update)
- Backend service with UTILIZATION mode
- Backend service with CONNECTION mode (for RegionalInternalExternal)

**Mocks Needed:**
- MockRegionBackendServices

**Complexity:** Medium - Similar to existing `TestService_createOrGetRegionalBackendService`

#### `TestService_createOrGetRegionalTargetTCPProxy` ⭐ **CRITICAL**
Tests regional target TCP proxy creation - the key missing piece for GCD.

**Test Cases:**
- Target TCP proxy does not exist (should create)
  - Uses RegionalKey (not GlobalKey)
  - Sets Region field
  - Sets Service field to backend service SelfLink
- Target TCP proxy exists (should get)
- Error handling when Get fails (non-404)
- Error handling when Insert fails

**Mocks Needed:**
- MockRegionTargetTcpProxies (NEW mock type needed)

**Complexity:** Medium - Similar to existing `TestService_createOrGetTargetTCPProxy`

#### `TestService_createOrGetRegionalAddress`
Tests regional address creation for external LB.

**Test Cases:**
- Address does not exist (should create)
  - Uses RegionalKey
  - Sets Region field
  - Uses EXTERNAL AddressType (not INTERNAL)
  - No Subnetwork field
  - No Purpose field
- Address exists (should get)
- Address with custom IP from config
- Address without custom IP (ephemeral)

**Mocks Needed:**
- MockAddresses (already exists, test regional usage)

**Complexity:** Medium - Similar to existing `TestService_createOrGetAddress`

#### `TestService_createOrGetRegionalForwardingRuleWithProxy`
Tests regional forwarding rule creation that points to target proxy.

**Test Cases:**
- Forwarding rule does not exist (should create)
  - Uses RegionalKey
  - Sets Region field
  - Points to Target (proxy) not BackendService
  - Uses EXTERNAL LoadBalancingScheme
  - Uses PortRange (not Ports array)
- Forwarding rule exists (should get)
- Label setting after creation
- Label update when labels differ

**Mocks Needed:**
- MockRegionForwardingRules (already exists)

**Complexity:** Medium - Similar to existing `TestService_createOrGetForwardingRule`

#### `TestService_createRegionalExternalLoadBalancer`
Tests the main orchestration function for regional external LB.

**Test Cases:**
- Creates all components in correct order
  - Regional health check
  - Regional backend service (EXTERNAL)
  - Regional target TCP proxy
  - Regional address
  - Regional forwarding rule
- Sets all network status fields correctly
- Sets control plane endpoint
- Uses custom name from ExternalLoadBalancer config
- Uses default name when config not provided
- UTILIZATION mode for RegionalExternal
- CONNECTION mode for RegionalInternalExternal
- Error handling at each step

**Mocks Needed:**
- All regional mocks (health check, backend service, target proxy, address, forwarding rule)

**Complexity:** High - Integration-style test with multiple mocks

---

### 3. Deletion Functions (3 tests needed)

#### `TestService_deleteRegionalAddress`
Tests regional address deletion.

**Test Cases:**
- Address exists (should delete)
- Address does not exist (should not error - 404 ok)
- Uses RegionalKey (not GlobalKey)

**Mocks Needed:**
- MockAddresses

**Complexity:** Low - Simple delete test

#### `TestService_deleteRegionalTargetTCPProxy`
Tests regional target TCP proxy deletion.

**Test Cases:**
- Target TCP proxy exists (should delete)
- Target TCP proxy does not exist (should not error - 404 ok)
- Uses RegionalKey (not GlobalKey)
- Error handling for non-404 errors

**Mocks Needed:**
- MockRegionTargetTcpProxies

**Complexity:** Low - Simple delete test

#### `TestService_deleteRegionalExternalLoadBalancer`
Tests deletion orchestration for regional external LB.

**Test Cases:**
- Deletes all components in reverse order
  1. Forwarding rule
  2. Address
  3. Target TCP proxy
  4. Backend service
  5. Health check
- Clears all network status fields
- Uses custom name from config
- Continues deleting even if one component fails
- Returns aggregated errors

**Mocks Needed:**
- All regional mocks

**Complexity:** High - Integration-style test

---

### 4. Modified Functions (2 tests needed)

#### `TestService_Reconcile` (needs update)
Existing test needs new cases for regional external LB.

**New Test Cases Needed:**
- RegionalExternal type creates regional external LB
- RegionalInternalExternal type creates both regional external and internal LBs
- Branching logic chooses regional vs global correctly
- Does not create regional LB for External type
- Does not create regional LB for Internal type

**Complexity:** Medium - Extend existing test

#### `TestService_Delete` (needs update)
Existing test needs new cases for regional external LB deletion.

**New Test Cases Needed:**
- RegionalExternal type deletes regional external LB
- RegionalInternalExternal type deletes both LBs
- Branching logic chooses regional vs global deletion
- Does not delete regional LB for External type

**Complexity:** Medium - Extend existing test

---

## Mock Types Needed

### New Mock Type Required

**`MockRegionTargetTcpProxies`** - Does not exist yet

This needs to be created similar to existing `MockTargetTcpProxies` but for regional resources.

**Required Interface:**
```go
type MockRegionTargetTcpProxies interface {
    Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*compute.TargetTcpProxy, error)
    Insert(ctx context.Context, key *meta.Key, obj *compute.TargetTcpProxy, options ...k8scloud.Option) error
    Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
}
```

### Existing Mocks to Reuse

These already exist in the test suite:
- `MockHealthChecks` (for regional health checks)
- `MockRegionBackendServices`
- `MockAddresses` (can handle regional addresses)
- `MockRegionForwardingRules`

---

## Test Coverage Summary

| Category | Tests Needed | Priority | Complexity |
|----------|--------------|----------|------------|
| Helper Functions | 2 | High | Low |
| Creation Functions | 5 | Critical | Medium-High |
| Deletion Functions | 3 | High | Low-High |
| Modified Functions | 2 | High | Medium |
| **TOTAL** | **12** | - | - |

---

## Critical Path Tests

If time is limited, prioritize these tests first:

1. **`TestService_createOrGetRegionalTargetTCPProxy`** ⭐ **MUST HAVE**
   - This is the critical new component that enables GCD support
   - No existing test coverage for regional target TCP proxies

2. **`TestIsRegionalExternalLoadBalancer`** - High value, low cost
   - Quick to write, tests branching logic

3. **`TestShouldCreateExternalLoadBalancer`** - High value, low cost
   - Quick to write, tests branching logic

4. **`TestService_createRegionalExternalLoadBalancer`** - Integration test
   - Tests the full flow end-to-end

5. **`TestService_Reconcile`** (update) - Ensures branching works
   - Tests that the right code path is taken

---

## Recommended Test Implementation Order

### Phase 1: Foundation (Low-hanging fruit)
1. `TestIsRegionalExternalLoadBalancer`
2. `TestShouldCreateExternalLoadBalancer`

### Phase 2: Core Components (Critical path)
3. Create `MockRegionTargetTcpProxies` mock type
4. `TestService_createOrGetRegionalTargetTCPProxy` ⭐
5. `TestService_createOrGetRegionalBackendServiceExternal`
6. `TestService_createOrGetRegionalAddress`
7. `TestService_createOrGetRegionalForwardingRuleWithProxy`

### Phase 3: Integration (High level)
8. `TestService_createRegionalExternalLoadBalancer`
9. Update `TestService_Reconcile`

### Phase 4: Cleanup (Deletion)
10. `TestService_deleteRegionalTargetTCPProxy`
11. `TestService_deleteRegionalAddress`
12. `TestService_deleteRegionalExternalLoadBalancer`
13. Update `TestService_Delete`

---

## Example Test Template

Here's an example test for `TestService_createOrGetRegionalTargetTCPProxy`:

```go
func TestService_createOrGetRegionalTargetTCPProxy(t *testing.T) {
	tests := []struct {
		name                       string
		scope                      func(s *scope.ClusterScope) Scope
		backendService             *compute.BackendService
		mockRegionalTargetTCPProxy *cloud.MockRegionTargetTcpProxies
		want                       *compute.TargetTcpProxy
		wantErr                    bool
	}{
		{
			name:  "regional target tcp proxy does not exist (should create)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			backendService: &compute.BackendService{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			},
			want: &compute.TargetTcpProxy{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				Service:  "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Region:   "us-central1",
			},
			wantErr: false,
		},
		{
			name:  "regional target tcp proxy exists (should get)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			backendService: &compute.BackendService{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {},
				},
			},
			want: &compute.TargetTcpProxy{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				Service:  "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Region:   "us-central1",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			s := New(tt.scope(clusterScope))
			s.regionaltargettcpproxies = tt.mockRegionalTargetTCPProxy
			got, err := s.createOrGetRegionalTargetTCPProxy(ctx, tt.backendService)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRegionalTargetTCPProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetRegionalTargetTCPProxy() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
```

---

## Estimated Effort

- **Helper function tests (2):** 1-2 hours
- **Creation function tests (5):** 8-10 hours
- **Deletion function tests (3):** 3-4 hours  
- **Modified function updates (2):** 2-3 hours
- **Mock type creation (1):** 1-2 hours

**Total Estimated Effort:** 15-21 hours

---

## Risk Assessment

### Without Tests

❌ **High Risk** - Changes are untested:
- No verification that RegionalKey is used correctly
- No verification that EXTERNAL scheme is set
- No verification that Region field is set
- No verification of error handling
- No verification of branching logic
- Refactoring is risky without tests

### With Tests

✅ **Low Risk** - Changes are verified:
- All critical paths tested
- Safe to refactor
- Catches regressions
- Documents expected behavior
- Enables confident code review
