# CORS-4448 Implementation Status

**Last Updated**: 2026-06-03  
**Current Branch**: CORS-4448  
**Status**: 🟡 **API Complete, Code Implementation Pending**

---

## Quick Status

```
API Changes:        ████████████████████ 100% ✅ COMPLETE (Commit a942318e)
Code Implementation: ░░░░░░░░░░░░░░░░░░░░   0% ❌ NOT STARTED
Unit Tests:         ░░░░░░░░░░░░░░░░░░░░   0% ❌ NOT STARTED (12 tests needed)
Integration Tests:  ░░░░░░░░░░░░░░░░░░░░   0% ❌ NOT STARTED

Overall Progress:   █████░░░░░░░░░░░░░░░  25%
```

---

## What's Done ✅

### 1. API Design (100% Complete)

**Commit**: a942318e - "api: Add support for regional external load balancers in GCD environments"

**Files Modified:**
- ✅ `api/v1beta1/types.go` - New enum values and configuration fields
- ✅ `api/v1beta1/zz_generated.deepcopy.go` - Auto-generated
- ✅ CRD manifests (4 files) - All updated with new enum values

**What Was Added:**

```go
// New LoadBalancerType enum values
RegionalExternal = LoadBalancerType("RegionalExternal")
RegionalInternalExternal = LoadBalancerType("RegionalInternalExternal")

// New configuration field in LoadBalancerSpec
ExternalLoadBalancer *LoadBalancer `json:"externalLoadBalancer,omitempty"`

// Enhanced LoadBalancer struct with proper documentation
type LoadBalancer struct {
    Name           *string        `json:"name,omitempty"`
    Subnet         *string        `json:"subnet,omitempty"`
    InternalAccess InternalAccess `json:"internalAccess,omitempty"`
    IPAddress      *string        `json:"ipAddress,omitempty"`
}
```

**Validation**: ✅ All kubebuilder annotations in place

### 2. Documentation (100% Complete)

**Files Created:**
- ✅ `docs/proposals/cors-4448/CORS-4448-proposal.md` (741 lines)
- ✅ `docs/proposals/cors-4448/CORS-4448-architecture.md` (415 lines)
- ✅ `docs/proposals/cors-4448/CORS-4448-quick-reference.md` (219 lines)
- ✅ `docs/proposals/cors-4448/CORS-4448-implementation-example.go` (554 lines)
- ✅ `docs/proposals/cors-4448/missing-unit-tests.md` (comprehensive test plan)
- ✅ `docs/proposals/cors-4448/PROPOSAL-REVIEW.md` (review document)

---

## What's Left ❌

### 1. Service Interface Updates (0% Complete)

**File**: `cloud/services/compute/loadbalancers/service.go`

**Required Changes:**

```go
// ADD: New interface
type regionaltargettcpproxiesInterface interface {
    Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*compute.TargetTcpProxy, error)
    Insert(ctx context.Context, key *meta.Key, obj *compute.TargetTcpProxy, options ...k8scloud.Option) error
    Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
}

// ADD: New field in Service struct
type Service struct {
    scope                    Scope
    addresses                addressesInterface
    internaladdresses        addressesInterface
    backendservices          backendservicesInterface
    regionalbackendservices  backendservicesInterface
    forwardingrules          forwardingrulesInterface
    regionalforwardingrules  regionalforwardingrulesInterface
    healthchecks             healthchecksInterface
    regionalhealthchecks     healthchecksInterface
    instancegroups           instancegroupsInterface
    targettcpproxies         targettcpproxiesInterface
    regionaltargettcpproxies regionaltargettcpproxiesInterface  // ← NEW
    subnets                  subnetsInterface
}

// UPDATE: New() function
func New(scope Scope) *Service {
    cloudScope := scope.Cloud()
    if scope.IsSharedVpc() {
        cloudScope = scope.NetworkCloud()
    }

    return &Service{
        // ... existing fields ...
        regionaltargettcpproxies: scope.Cloud().RegionTargetTcpProxies(), // ← NEW
        // ...
    }
}
```

**Estimated Effort**: 1 hour

---

### 2. Helper Functions (0% Complete)

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

**Functions to Add:**

```go
// isRegionalExternalLoadBalancer returns true if the load balancer type
// requires regional external resources.
func isRegionalExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
    return lbType == infrav1.RegionalExternal || 
           lbType == infrav1.RegionalInternalExternal
}

// shouldCreateExternalLoadBalancer returns true if an external load balancer
// should be created.
func shouldCreateExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
    return lbType == infrav1.External || 
           lbType == infrav1.InternalExternal ||
           lbType == infrav1.RegionalExternal ||
           lbType == infrav1.RegionalInternalExternal
}
```

**Estimated Effort**: 30 minutes

---

### 3. Creation Functions (0% Complete)

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

**Functions to Add (5 total):**

| Function | Priority | Complexity | Estimated Time |
|----------|----------|------------|----------------|
| `createRegionalExternalLoadBalancer()` | ⭐ Critical | High | 2-3 hours |
| `createOrGetRegionalBackendServiceExternal()` | High | Medium | 1-2 hours |
| `createOrGetRegionalTargetTCPProxy()` | ⭐ Critical | Medium | 1-2 hours |
| `createOrGetRegionalAddress()` | High | Medium | 1 hour |
| `createOrGetRegionalForwardingRuleWithProxy()` | High | Medium | 1-2 hours |

**Total Estimated Effort**: 6-10 hours

**Reference**: See `CORS-4448-implementation-example.go` for complete code examples

---

### 4. Deletion Functions (0% Complete)

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

**Functions to Add (3 total):**

| Function | Complexity | Estimated Time |
|----------|------------|----------------|
| `deleteRegionalExternalLoadBalancer()` | Medium | 1 hour |
| `deleteRegionalTargetTCPProxy()` | Low | 30 minutes |
| `deleteRegionalAddress()` | Low | 30 minutes |

**Total Estimated Effort**: 2 hours

---

### 5. Modified Functions (0% Complete)

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

**Functions to Modify (2 total):**

#### Reconcile() Function

**Current Logic:**
```go
if lbType == infrav1.External || lbType == infrav1.InternalExternal {
    if err = s.createExternalLoadBalancer(ctx, lbType, instancegroups); err != nil {
        return err
    }
}
```

**New Logic:**
```go
if shouldCreateExternalLoadBalancer(lbType) {
    if isRegionalExternalLoadBalancer(lbType) {
        if err = s.createRegionalExternalLoadBalancer(ctx, lbType, instancegroups); err != nil {
            return err
        }
    } else {
        if err = s.createExternalLoadBalancer(ctx, lbType, instancegroups); err != nil {
            return err
        }
    }
}
```

**Estimated Effort**: 1 hour

#### Delete() Function

**Current Logic:**
```go
if lbType == infrav1.External || lbType == infrav1.InternalExternal {
    if err := s.deleteExternalLoadBalancer(ctx); err != nil {
        allErrs = append(allErrs, err)
    }
}
```

**New Logic:**
```go
if shouldCreateExternalLoadBalancer(lbType) {
    if isRegionalExternalLoadBalancer(lbType) {
        if err := s.deleteRegionalExternalLoadBalancer(ctx); err != nil {
            allErrs = append(allErrs, err)
        }
    } else {
        if err := s.deleteExternalLoadBalancer(ctx); err != nil {
            allErrs = append(allErrs, err)
        }
    }
}
```

**Estimated Effort**: 1 hour

---

### 6. Unit Tests (0% Complete)

**File**: `cloud/services/compute/loadbalancers/reconcile_test.go`

**Tests Needed (12 total):**

#### Phase 1: Foundation Tests (2 tests)
- [ ] `TestIsRegionalExternalLoadBalancer` (30 min)
- [ ] `TestShouldCreateExternalLoadBalancer` (30 min)

#### Phase 2: Core Component Tests (5 tests)
- [ ] `TestService_createOrGetRegionalBackendServiceExternal` (2 hours)
- [ ] ⭐ `TestService_createOrGetRegionalTargetTCPProxy` (2 hours) **CRITICAL**
- [ ] `TestService_createOrGetRegionalAddress` (1.5 hours)
- [ ] `TestService_createOrGetRegionalForwardingRuleWithProxy` (1.5 hours)
- [ ] `TestService_createRegionalExternalLoadBalancer` (2 hours)

#### Phase 3: Deletion Tests (3 tests)
- [ ] `TestService_deleteRegionalAddress` (1 hour)
- [ ] `TestService_deleteRegionalTargetTCPProxy` (1 hour)
- [ ] `TestService_deleteRegionalExternalLoadBalancer` (1.5 hours)

#### Phase 4: Modified Function Tests (2 tests)
- [ ] `TestService_Reconcile` - Add new test cases (1.5 hours)
- [ ] `TestService_Delete` - Add new test cases (1.5 hours)

**Additional Work:**
- [ ] Create `MockRegionTargetTcpProxies` mock type (2 hours)

**Total Estimated Effort**: 17-20 hours

**Reference**: See `missing-unit-tests.md` for detailed test specifications

---

### 7. Integration Tests (0% Complete)

**Tests Needed (3 total):**

- [ ] Regional external LB lifecycle test
- [ ] `RegionalInternalExternal` dual LB test
- [ ] Resource cleanup verification test

**Estimated Effort**: 6-8 hours

---

## Implementation Roadmap

### Phase 1: Foundation (1-2 days) ✋ START HERE
**Goal**: Set up interfaces and helper functions

**Tasks:**
1. Update `service.go`:
   - Add `regionaltargettcpproxiesInterface`
   - Add field to `Service` struct
   - Update `New()` function
2. Add helper functions to `reconcile.go`:
   - `isRegionalExternalLoadBalancer()`
   - `shouldCreateExternalLoadBalancer()`
3. Write unit tests for helpers

**Deliverable**: Helpers in place with tests passing

---

### Phase 2: Core Creation (3-5 days)
**Goal**: Implement regional external LB creation

**Tasks:**
1. Implement `createOrGetRegionalTargetTCPProxy()` ⭐
2. Implement `createOrGetRegionalBackendServiceExternal()`
3. Implement `createOrGetRegionalAddress()`
4. Implement `createOrGetRegionalForwardingRuleWithProxy()`
5. Implement `createRegionalExternalLoadBalancer()` (orchestration)
6. Write unit tests for each function

**Deliverable**: Regional external LB can be created

---

### Phase 3: Reconciliation (1-2 days)
**Goal**: Wire up creation logic to Reconcile()

**Tasks:**
1. Update `Reconcile()` with branching logic
2. Test manually with `RegionalExternal` type
3. Test manually with `RegionalInternalExternal` type
4. Update `TestService_Reconcile` with new test cases

**Deliverable**: Reconcile() properly creates regional external LBs

---

### Phase 4: Deletion (1-2 days)
**Goal**: Implement cleanup logic

**Tasks:**
1. Implement `deleteRegionalTargetTCPProxy()`
2. Implement `deleteRegionalAddress()`
3. Implement `deleteRegionalExternalLoadBalancer()`
4. Update `Delete()` with branching logic
5. Write unit tests for deletion functions
6. Update `TestService_Delete` with new test cases

**Deliverable**: Regional external LBs can be deleted cleanly

---

### Phase 5: Testing (3-5 days)
**Goal**: Achieve comprehensive test coverage

**Tasks:**
1. Create `MockRegionTargetTcpProxies` mock type
2. Write all remaining unit tests (see Phase list above)
3. Verify test coverage meets goals (>90%)
4. Fix any failing tests

**Deliverable**: All unit tests passing with high coverage

---

### Phase 6: Integration & Validation (2-3 days)
**Goal**: Verify in real GCD environment

**Tasks:**
1. Write integration tests
2. Test in GCD environment (if available)
3. Test resource creation and cleanup
4. Verify control plane endpoint accessibility
5. Performance testing

**Deliverable**: Feature verified in GCD environment

---

## Total Effort Estimate

| Phase | Estimated Time |
|-------|----------------|
| Phase 1: Foundation | 1-2 days |
| Phase 2: Core Creation | 3-5 days |
| Phase 3: Reconciliation | 1-2 days |
| Phase 4: Deletion | 1-2 days |
| Phase 5: Testing | 3-5 days |
| Phase 6: Integration | 2-3 days |
| **Total** | **11-19 days** |

---

## Current Blockers

### ❌ No Blockers

All prerequisites are in place:
- ✅ API design complete
- ✅ Documentation complete
- ✅ GCP API support verified (`RegionTargetTcpProxies()` available)
- ✅ Implementation examples provided
- ✅ Test plan defined

---

## Next Steps

### Immediate (Today)
1. Review `PROPOSAL-REVIEW.md` 
2. Review implementation examples in `CORS-4448-implementation-example.go`
3. Set up development environment

### This Week
1. Start Phase 1: Foundation
   - Update `service.go`
   - Add helper functions
   - Write helper tests
2. Begin Phase 2: Core Creation
   - Implement `createOrGetRegionalTargetTCPProxy()` ⭐
   - Implement remaining creation functions

### Next Week
1. Complete Phase 2
2. Complete Phase 3: Reconciliation
3. Start Phase 4: Deletion

### Following Weeks
1. Complete Phase 4
2. Complete Phase 5: Testing
3. Complete Phase 6: Integration

---

## Success Criteria

### Minimum Viable Implementation
- [x] API changes complete ✅
- [ ] `RegionalExternal` type creates regional external LB
- [ ] All components use `meta.RegionalKey()`
- [ ] Control plane endpoint accessible
- [ ] Deletion cleans up all regional resources
- [ ] Unit tests pass

### Full Implementation
- [ ] All above ✓
- [ ] `RegionalInternalExternal` type creates dual LBs
- [ ] Integration tests pass
- [ ] Tested in GCD environment
- [ ] >90% code coverage
- [ ] Documentation updated

---

## Resources

### Documentation
- **Main Proposal**: `CORS-4448-proposal.md`
- **Architecture**: `CORS-4448-architecture.md`
- **Quick Reference**: `CORS-4448-quick-reference.md`
- **Implementation Examples**: `CORS-4448-implementation-example.go`
- **Test Plan**: `missing-unit-tests.md`
- **Review**: `PROPOSAL-REVIEW.md`

### Code References
- **Existing Pattern**: `createInternalLoadBalancer()` in `reconcile.go` (regional internal LB)
- **Global Pattern**: `createExternalLoadBalancer()` in `reconcile.go` (global external LB)
- **Service Setup**: `service.go` for interface patterns

### External Resources
- **Jira Ticket**: [CORS-4448](https://redhat.atlassian.net/browse/CORS-4448)
- **GCP Docs**: [Regional External Load Balancers](https://cloud.google.com/load-balancing/docs/tcp#regional)

---

## Questions or Issues?

If you encounter any issues during implementation:

1. Check the implementation examples in `CORS-4448-implementation-example.go`
2. Review existing patterns in `reconcile.go`
3. Consult the test plan in `missing-unit-tests.md`
4. Reference the architecture diagrams in `CORS-4448-architecture.md`

---

**Last Updated**: 2026-06-03  
**Next Review**: After Phase 1 completion
