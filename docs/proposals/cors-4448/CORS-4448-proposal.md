# CORS-4448: Regional External Load Balancer Support for GCD

## Problem Statement

Current CAPG implementation creates **Global External Proxy Load Balancers** by default, which are not compatible with GCD (Google Cloud Distributed/Sovereign Cloud) environments. GCD requires **Regional External Load Balancers** instead.

The main issue is in `createExternalLoadBalancer()` in `cloud/services/compute/loadbalancers/reconcile.go` where all 5 LB components use `meta.GlobalKey()`:
- Health Check
- Backend Service  
- Target TCP Proxy
- Address
- Forwarding Rule

## Proposed Solution

### Part 1: API Changes

#### 1.1 Add New LoadBalancerType Enum Value

**File**: `api/v1beta1/types.go`

Add a new LoadBalancerType for regional external load balancers:

```go
// LoadBalancerType defines the Load Balancer that should be created.
type LoadBalancerType string

var (
    // External creates a Global External Proxy Load Balancer
    // to manage traffic to backends in multiple regions. This is the default Load
    // Balancer and will be created if no LoadBalancerType is defined.
    External = LoadBalancerType("External")

    // RegionalExternal creates a Regional External Proxy Load Balancer
    // to manage traffic to backends in a single region. This is required for
    // GCD (Google Cloud Distributed/Sovereign Cloud) environments.
    RegionalExternal = LoadBalancerType("RegionalExternal")

    // Internal creates a Regional Internal Passthrough Load
    // Balancer to manage traffic to backends in the configured region.
    Internal = LoadBalancerType("Internal")

    // InternalExternal creates both External and Internal Load Balancers to provide
    // separate endpoints for managing both external and internal traffic.
    InternalExternal = LoadBalancerType("InternalExternal")
    
    // RegionalInternalExternal creates both RegionalExternal and Internal Load Balancers
    // to provide separate endpoints for managing both external and internal traffic in
    // GCD environments.
    RegionalInternalExternal = LoadBalancerType("RegionalInternalExternal")
)
```

**Rationale**: 
- `RegionalExternal` - Explicitly indicates regional scope for external load balancers
- `RegionalInternalExternal` - Supports dual LB setup in GCD environments
- Backward compatible - existing `External` remains default

#### 1.2 Add External Load Balancer Configuration

**File**: `api/v1beta1/types.go`

Enhance `LoadBalancerSpec` to support external LB configuration similar to internal:

```go
// LoadBalancerSpec contains configuration for one or more LoadBalancers.
type LoadBalancerSpec struct {
    // APIServerInstanceGroupTagOverride overrides the default setting for the
    // tag used when creating the API Server Instance Group.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxLength=16
    // +kubebuilder:validation:Pattern=`(^[1-9][0-9]{0,31}$)|(^[a-z][a-z0-9-]{4,28}[a-z0-9]$)`
    // +optional
    APIServerInstanceGroupTagOverride *string `json:"apiServerInstanceGroupTagOverride,omitempty"`

    // LoadBalancerType defines the type of Load Balancer that should be created.
    // If not set, a Global External Proxy Load Balancer will be created by default.
    // +optional
    LoadBalancerType *LoadBalancerType `json:"loadBalancerType,omitempty"`

    // ExternalLoadBalancer is the configuration for an External Proxy Load Balancer.
    // Only applicable when LoadBalancerType is RegionalExternal or RegionalInternalExternal.
    // +optional
    ExternalLoadBalancer *LoadBalancer `json:"externalLoadBalancer,omitempty"`

    // InternalLoadBalancer is the configuration for an Internal Passthrough Network Load Balancer.
    // +optional
    InternalLoadBalancer *LoadBalancer `json:"internalLoadBalancer,omitempty"`
}

// LoadBalancer defines the configuration for a Load Balancer.
type LoadBalancer struct {
    // Name allows you to set a custom name for the Load Balancer resources.
    // +optional
    Name *string `json:"name,omitempty"`

    // IPAddress allows you to set a custom IP address for the Load Balancer.
    // If not set, an ephemeral IP will be automatically allocated.
    // +optional
    IPAddress *string `json:"ipAddress,omitempty"`

    // Subnet allows you to set a custom subnet for regional Load Balancers.
    // Only applicable for regional Load Balancers (not global).
    // +optional
    Subnet *string `json:"subnet,omitempty"`

    // InternalAccess allows global access for Internal Load Balancers.
    // Only applicable for Internal Load Balancers.
    // +optional
    InternalAccess InternalAccess `json:"internalAccess,omitempty"`
}
```

**Rationale**:
- Consistent API pattern between internal and external LB configuration
- Allows users to specify custom names, IPs, and subnets for regional external LBs
- Future-proof for additional configuration needs

### Part 2: Software Changes

#### 2.1 Detect Load Balancer Scope

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

Add helper function to determine if LB should be regional or global:

```go
// isRegionalExternalLoadBalancer returns true if the load balancer type requires regional external resources.
func isRegionalExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
    return lbType == infrav1.RegionalExternal || lbType == infrav1.RegionalInternalExternal
}

// shouldCreateExternalLoadBalancer returns true if an external load balancer should be created.
func shouldCreateExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
    return lbType == infrav1.External || 
           lbType == infrav1.InternalExternal ||
           lbType == infrav1.RegionalExternal ||
           lbType == infrav1.RegionalInternalExternal
}
```

#### 2.2 Update Reconcile Logic

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

Modify the `Reconcile` function to branch based on LB scope:

```go
func (s *Service) Reconcile(ctx context.Context) error {
    log := log.FromContext(ctx)
    log.Info("Reconciling loadbalancer resources")

    // Creates instance groups used by load balancer(s)
    instancegroups, err := s.createOrGetInstanceGroups(ctx)
    if err != nil {
        return err
    }

    lbSpec := s.scope.LoadBalancer()
    lbType := ptr.Deref(lbSpec.LoadBalancerType, infrav1.External)
    
    // Create External Load Balancer (Global or Regional based on type)
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

    // Create Regional Internal Passthrough Load Balancer if configured
    if lbType == infrav1.Internal || 
       lbType == infrav1.InternalExternal ||
       lbType == infrav1.RegionalInternalExternal {
        name := infrav1.InternalRoleTagValue
        if lbSpec.InternalLoadBalancer != nil {
            name = ptr.Deref(lbSpec.InternalLoadBalancer.Name, infrav1.InternalRoleTagValue)
        }
        if err = s.createInternalLoadBalancer(ctx, name, lbType, instancegroups); err != nil {
            return err
        }
    }

    return nil
}
```

#### 2.3 Implement Regional External Load Balancer Creation

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

Add new function to create regional external load balancer:

```go
// createRegionalExternalLoadBalancer creates the components for a Regional External Proxy LoadBalancer.
// This is required for GCD (Google Cloud Distributed/Sovereign Cloud) environments.
func (s *Service) createRegionalExternalLoadBalancer(ctx context.Context, lbType infrav1.LoadBalancerType, instancegroups []*compute.InstanceGroup) error {
    lbSpec := s.scope.LoadBalancer()
    name := infrav1.APIServerRoleTagValue
    if lbSpec.ExternalLoadBalancer != nil && lbSpec.ExternalLoadBalancer.Name != nil {
        name = *lbSpec.ExternalLoadBalancer.Name
    }

    // Create regional health check
    healthcheck, err := s.createOrGetRegionalHealthCheck(ctx, name)
    if err != nil {
        return err
    }
    s.scope.Network().APIServerHealthCheck = ptr.To[string](healthcheck.SelfLink)

    // If an Internal LoadBalancer is being created, the BalancingMode must match
    mode := loadBalancingModeUtilization
    if lbType == infrav1.RegionalInternalExternal {
        mode = loadBalancingModeConnection
    }

    // Create regional backend service
    backendsvc, err := s.createOrGetRegionalBackendServiceExternal(ctx, name, mode, instancegroups, healthcheck)
    if err != nil {
        return err
    }
    s.scope.Network().APIServerBackendService = ptr.To[string](backendsvc.SelfLink)

    // Create regional TargetTCPProxy
    target, err := s.createOrGetRegionalTargetTCPProxy(ctx, backendsvc)
    if err != nil {
        return err
    }
    s.scope.Network().APIServerTargetProxy = ptr.To[string](target.SelfLink)

    // Create regional address
    addr, err := s.createOrGetRegionalAddress(ctx, name)
    if err != nil {
        return err
    }
    s.scope.Network().APIServerAddress = ptr.To[string](addr.SelfLink)
    endpoint := s.scope.ControlPlaneEndpoint()
    endpoint.Host = addr.Address
    s.scope.SetControlPlaneEndpoint(endpoint)

    // Create regional forwarding rule with proxy
    forwarding, err := s.createOrGetRegionalForwardingRuleWithProxy(ctx, name, target, addr)
    if err != nil {
        return err
    }
    s.scope.Network().APIServerForwardingRule = ptr.To[string](forwarding.SelfLink)

    return nil
}
```

#### 2.4 Implement Regional Backend Service for External LB

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

```go
// createOrGetRegionalBackendServiceExternal creates a regional backend service for external load balancers.
// This is different from the internal regional backend service which uses INTERNAL load balancing scheme.
func (s *Service) createOrGetRegionalBackendServiceExternal(ctx context.Context, lbname string, mode loadBalancingMode, instancegroups []*compute.InstanceGroup, healthcheck *compute.HealthCheck) (*compute.BackendService, error) {
    log := log.FromContext(ctx)
    backends := make([]*compute.Backend, 0, len(instancegroups))
    for _, group := range instancegroups {
        be := &compute.Backend{
            BalancingMode: string(mode),
            Group:         group.SelfLink,
        }
        if mode == loadBalancingModeConnection {
            be.MaxConnections = 1000
        }
        backends = append(backends, be)
    }

    backendsvcSpec := s.scope.BackendServiceSpec(lbname)
    backendsvcSpec.Backends = backends
    backendsvcSpec.HealthChecks = []string{healthcheck.SelfLink}
    backendsvcSpec.Region = s.scope.Region()
    // External regional load balancer uses EXTERNAL scheme (not INTERNAL)
    backendsvcSpec.LoadBalancingScheme = "EXTERNAL"

    key := meta.RegionalKey(backendsvcSpec.Name, s.scope.Region())
    backendsvc, err := s.regionalbackendservices.Get(ctx, key)
    if err != nil {
        if !gcperrors.IsNotFound(err) {
            log.Error(err, "Error looking for regional backend service", "name", backendsvcSpec.Name)
            return nil, err
        }

        log.V(2).Info("Creating a regional backend service for external LB", "name", backendsvcSpec.Name)
        if err := s.regionalbackendservices.Insert(ctx, key, backendsvcSpec); err != nil {
            log.Error(err, "Error creating a regional backend service", "name", backendsvcSpec.Name)
            return nil, err
        }

        backendsvc, err = s.regionalbackendservices.Get(ctx, key)
        if err != nil {
            return nil, err
        }
    }

    if len(backendsvc.Backends) != len(backendsvcSpec.Backends) {
        log.V(2).Info("Updating a regional backend service", "name", backendsvcSpec.Name)
        backendsvc.Backends = backendsvcSpec.Backends
        if err := s.regionalbackendservices.Update(ctx, key, backendsvc); err != nil {
            log.Error(err, "Error updating a regional backend service", "name", backendsvcSpec.Name)
            return nil, err
        }
    }

    return backendsvc, nil
}
```

#### 2.5 Implement Regional Target TCP Proxy (CRITICAL)

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

```go
// createOrGetRegionalTargetTCPProxy creates or gets a regional TargetTCPProxy.
// This is the CRITICAL missing piece for regional external load balancers in GCD.
func (s *Service) createOrGetRegionalTargetTCPProxy(ctx context.Context, service *compute.BackendService) (*compute.TargetTcpProxy, error) {
    log := log.FromContext(ctx)
    targetSpec := s.scope.TargetTCPProxySpec()
    targetSpec.Service = service.SelfLink
    targetSpec.Region = s.scope.Region()
    
    key := meta.RegionalKey(targetSpec.Name, s.scope.Region())
    target, err := s.regionaltargettcpproxies.Get(ctx, key)
    if err != nil {
        if !gcperrors.IsNotFound(err) {
            log.Error(err, "Error looking for regional targettcpproxy", "name", targetSpec.Name)
            return nil, err
        }

        log.V(2).Info("Creating a regional targettcpproxy", "name", targetSpec.Name)
        if err := s.regionaltargettcpproxies.Insert(ctx, key, targetSpec); err != nil {
            log.Error(err, "Error creating a regional targettcpproxy", "name", targetSpec.Name)
            return nil, err
        }

        target, err = s.regionaltargettcpproxies.Get(ctx, key)
        if err != nil {
            return nil, err
        }
    }

    return target, nil
}
```

#### 2.6 Implement Regional Address for External LB

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

```go
// createOrGetRegionalAddress creates or gets a regional address for external load balancers.
// This is different from createOrGetInternalAddress which sets AddressType to INTERNAL.
func (s *Service) createOrGetRegionalAddress(ctx context.Context, lbname string) (*compute.Address, error) {
    log := log.FromContext(ctx)
    addrSpec := s.scope.AddressSpec(lbname)
    addrSpec.Region = s.scope.Region()
    addrSpec.AddressType = "EXTERNAL"
    
    lbSpec := s.scope.LoadBalancer()
    if lbSpec.ExternalLoadBalancer != nil && lbSpec.ExternalLoadBalancer.IPAddress != nil {
        addrSpec.Address = *lbSpec.ExternalLoadBalancer.IPAddress
    }

    log.V(2).Info("Looking for regional external address", "name", addrSpec.Name)
    key := meta.RegionalKey(addrSpec.Name, s.scope.Region())
    addr, err := s.addresses.Get(ctx, key)
    if err != nil {
        if !gcperrors.IsNotFound(err) {
            log.Error(err, "Error looking for regional external address", "name", addrSpec.Name)
            return nil, err
        }

        log.V(2).Info("Creating a regional external address", "name", addrSpec.Name)
        if err := s.addresses.Insert(ctx, key, addrSpec); err != nil {
            log.Error(err, "Error creating a regional external address", "name", addrSpec.Name)
            return nil, err
        }

        addr, err = s.addresses.Get(ctx, key)
        if err != nil {
            return nil, err
        }
    }

    return addr, nil
}
```

#### 2.7 Implement Regional Forwarding Rule with Proxy

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

```go
// createOrGetRegionalForwardingRuleWithProxy creates a regional forwarding rule that points to a target proxy.
// This is for regional external load balancers, different from createOrGetRegionalForwardingRule which
// is for internal passthrough load balancers that point directly to a backend service.
func (s *Service) createOrGetRegionalForwardingRuleWithProxy(ctx context.Context, lbname string, target *compute.TargetTcpProxy, addr *compute.Address) (*compute.ForwardingRule, error) {
    log := log.FromContext(ctx)
    spec := s.scope.ForwardingRuleSpec(lbname)
    spec.Region = s.scope.Region()
    spec.Target = target.SelfLink
    spec.IPAddress = addr.SelfLink
    spec.LoadBalancingScheme = "EXTERNAL"

    key := meta.RegionalKey(spec.Name, s.scope.Region())
    log.V(2).Info("Looking for regional forwarding rule with proxy", "name", spec.Name)
    forwarding, err := s.regionalforwardingrules.Get(ctx, key)
    if err != nil {
        if !gcperrors.IsNotFound(err) {
            log.Error(err, "Error looking for regional forwarding rule", "name", spec.Name)
            return nil, err
        }

        log.V(2).Info("Creating a regional forwarding rule with proxy", "name", spec.Name)
        if err := s.regionalforwardingrules.Insert(ctx, key, spec); err != nil {
            log.Error(err, "Error creating a regional forwarding rule", "name", spec.Name)
            return nil, err
        }

        forwarding, err = s.regionalforwardingrules.Get(ctx, key)
        if err != nil {
            return nil, err
        }
    }

    // Labels on ForwardingRules must be added after resource is created
    labels := s.scope.AdditionalLabels()
    if !labels.Equals(forwarding.Labels) {
        setLabelsRequest := &compute.RegionSetLabelsRequest{
            LabelFingerprint: forwarding.LabelFingerprint,
            Labels:           labels,
        }
        if err = s.regionalforwardingrules.SetLabels(ctx, key, setLabelsRequest); err != nil {
            return nil, err
        }
    }

    return forwarding, nil
}
```

#### 2.8 Add Regional Target TCP Proxy Interface

**File**: `cloud/services/compute/loadbalancers/service.go`

```go
type regionaltargettcpproxiesInterface interface {
    Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*compute.TargetTcpProxy, error)
    Insert(ctx context.Context, key *meta.Key, obj *compute.TargetTcpProxy, options ...k8scloud.Option) error
    Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
}

// Service implements loadbalancers reconciler.
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
    regionaltargettcpproxies regionaltargettcpproxiesInterface  // NEW
    subnets                  subnetsInterface
}
```

Update the `New` function:

```go
func New(scope Scope) *Service {
    cloudScope := scope.Cloud()
    if scope.IsSharedVpc() {
        cloudScope = scope.NetworkCloud()
    }

    return &Service{
        scope:                    scope,
        addresses:                scope.Cloud().GlobalAddresses(),
        internaladdresses:        scope.Cloud().Addresses(),
        backendservices:          scope.Cloud().BackendServices(),
        regionalbackendservices:  scope.Cloud().RegionBackendServices(),
        forwardingrules:          scope.Cloud().GlobalForwardingRules(),
        regionalforwardingrules:  scope.Cloud().ForwardingRules(),
        healthchecks:             scope.Cloud().HealthChecks(),
        regionalhealthchecks:     scope.Cloud().RegionHealthChecks(),
        instancegroups:           scope.Cloud().InstanceGroups(),
        targettcpproxies:         scope.Cloud().TargetTcpProxies(),
        regionaltargettcpproxies: scope.Cloud().RegionTargetTcpProxies(), // NEW
        subnets:                  cloudScope.Subnetworks(),
    }
}
```

#### 2.9 Update Delete Logic

**File**: `cloud/services/compute/loadbalancers/reconcile.go`

```go
func (s *Service) Delete(ctx context.Context) error {
    log := log.FromContext(ctx)
    var allErrs []error
    lbSpec := s.scope.LoadBalancer()
    lbType := ptr.Deref(lbSpec.LoadBalancerType, infrav1.External)
    
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

    if lbType == infrav1.Internal || 
       lbType == infrav1.InternalExternal ||
       lbType == infrav1.RegionalInternalExternal {
        name := infrav1.InternalRoleTagValue
        if lbSpec.InternalLoadBalancer != nil {
            name = ptr.Deref(lbSpec.InternalLoadBalancer.Name, infrav1.InternalRoleTagValue)
        }
        if err := s.deleteInternalLoadBalancer(ctx, name); err != nil {
            allErrs = append(allErrs, err)
        }
    }
    
    if err := s.deleteInstanceGroups(ctx); err != nil {
        log.Error(err, "Error deleting instancegroup")
        allErrs = append(allErrs, err)
    }

    return errors.Join(allErrs...)
}

func (s *Service) deleteRegionalExternalLoadBalancer(ctx context.Context) error {
    log := log.FromContext(ctx)
    log.Info("Deleting regional external loadbalancer resources")
    lbSpec := s.scope.LoadBalancer()
    name := infrav1.APIServerRoleTagValue
    if lbSpec.ExternalLoadBalancer != nil && lbSpec.ExternalLoadBalancer.Name != nil {
        name = *lbSpec.ExternalLoadBalancer.Name
    }

    if err := s.deleteRegionalForwardingRule(ctx, name); err != nil {
        return fmt.Errorf("deleting Regional ForwardingRule: %w", err)
    }
    s.scope.Network().APIServerForwardingRule = nil

    if err := s.deleteRegionalAddress(ctx, name); err != nil {
        return fmt.Errorf("deleting Regional Address: %w", err)
    }
    s.scope.Network().APIServerAddress = nil

    if err := s.deleteRegionalTargetTCPProxy(ctx); err != nil {
        return fmt.Errorf("deleting Regional TargetTCPProxy: %w", err)
    }
    s.scope.Network().APIServerTargetProxy = nil

    if err := s.deleteRegionalBackendService(ctx, name); err != nil {
        return fmt.Errorf("deleting Regional BackendService: %w", err)
    }
    s.scope.Network().APIServerBackendService = nil

    if err := s.deleteRegionalHealthCheck(ctx, name); err != nil {
        return fmt.Errorf("deleting Regional HealthCheck: %w", err)
    }
    s.scope.Network().APIServerHealthCheck = nil

    return nil
}

func (s *Service) deleteRegionalAddress(ctx context.Context, lbname string) error {
    log := log.FromContext(ctx)
    spec := s.scope.AddressSpec(lbname)
    key := meta.RegionalKey(spec.Name, s.scope.Region())
    log.V(2).Info("Deleting a regional address", "name", spec.Name)
    if err := s.addresses.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
        return err
    }
    return nil
}

func (s *Service) deleteRegionalTargetTCPProxy(ctx context.Context) error {
    log := log.FromContext(ctx)
    spec := s.scope.TargetTCPProxySpec()
    key := meta.RegionalKey(spec.Name, s.scope.Region())
    log.V(2).Info("Deleting a regional targettcpproxy", "name", spec.Name)
    if err := s.regionaltargettcpproxies.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
        log.Error(err, "Error deleting a regional targettcpproxy", "name", spec.Name)
        return err
    }
    return nil
}
```

## Migration Path

### For Existing Clusters

Existing clusters using `External` LoadBalancerType will continue to work without changes (backward compatible).

### For New GCD Clusters

Users deploying to GCD environments should set:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  loadBalancer:
    loadBalancerType: RegionalExternal
```

### For Dual LB in GCD

Users needing both external and internal LBs in GCD:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  loadBalancer:
    loadBalancerType: RegionalInternalExternal
    externalLoadBalancer:
      name: "custom-external-lb"
      ipAddress: "10.1.2.3"
    internalLoadBalancer:
      name: "custom-internal-lb"
      subnet: "control-plane-subnet"
```

## Testing Requirements

### Unit Tests

1. Test `isRegionalExternalLoadBalancer()` function
2. Test `shouldCreateExternalLoadBalancer()` function
3. Test `createRegionalExternalLoadBalancer()` flow
4. Test `deleteRegionalExternalLoadBalancer()` flow
5. Test regional resource creation (health check, backend service, target proxy, address, forwarding rule)

### Integration Tests

1. Create cluster with `RegionalExternal` load balancer type
2. Verify all components use `meta.RegionalKey()`
3. Verify control plane endpoint is accessible
4. Test deletion of regional resources
5. Test `RegionalInternalExternal` configuration

## Implementation Checklist

- [ ] Update API types (`api/v1beta1/types.go`)
- [ ] Add `RegionalExternal` and `RegionalInternalExternal` enum values
- [ ] Add `ExternalLoadBalancer` field to `LoadBalancerSpec`
- [ ] Update `LoadBalancer` type to support external LB configuration
- [ ] Add regional target TCP proxy interface (`service.go`)
- [ ] Implement `createRegionalExternalLoadBalancer()` (`reconcile.go`)
- [ ] Implement `createOrGetRegionalBackendServiceExternal()` (`reconcile.go`)
- [ ] Implement `createOrGetRegionalTargetTCPProxy()` (`reconcile.go`) - **CRITICAL**
- [ ] Implement `createOrGetRegionalAddress()` (`reconcile.go`)
- [ ] Implement `createOrGetRegionalForwardingRuleWithProxy()` (`reconcile.go`)
- [ ] Update `Reconcile()` to branch on LB type (`reconcile.go`)
- [ ] Implement `deleteRegionalExternalLoadBalancer()` (`reconcile.go`)
- [ ] Implement deletion functions for regional resources (`reconcile.go`)
- [ ] Update `Delete()` to handle regional resources (`reconcile.go`)
- [ ] Add unit tests (`reconcile_test.go`)
- [ ] Add integration tests (e2e)
- [ ] Update CRD manifests
- [ ] Update documentation

## Risks and Considerations

### API Compatibility

- **Risk**: Introducing new LoadBalancerType values
- **Mitigation**: Existing `External` type remains default; new types are opt-in

### GCP API Support

- **Risk**: Regional Target TCP Proxies may not be available in all GCP environments
- **Consideration**: Verify GCP Compute API supports `RegionTargetTcpProxies` in k8s-cloud-provider library

### Resource Naming

- **Risk**: Name collisions between global and regional resources
- **Mitigation**: Regional resources naturally have different resource paths (regional vs global)

### Migration Complexity

- **Risk**: Users may want to migrate existing global LBs to regional
- **Consideration**: This is out of scope - users should create new clusters with regional LBs

## Alternative Approaches Considered

### 1. Auto-detect GCD Environment

Instead of adding new LoadBalancerType values, automatically detect GCD environment and create regional LBs.

**Pros**: Simpler API, automatic behavior
**Cons**: Less explicit, harder to test, may surprise users

**Decision**: Rejected - prefer explicit configuration

### 2. Add Boolean Flag (e.g., `useRegionalResources`)

Add a simple boolean flag to switch between global and regional.

**Pros**: Simpler than new enum values
**Cons**: Less descriptive, harder to support mixed scenarios

**Decision**: Rejected - new enum values are more explicit and extensible

### 3. Reuse Existing `External` Type with Configuration

Keep `External` type but add configuration to specify global vs regional.

**Pros**: No API changes needed
**Cons**: Breaking change for existing users, less clear semantics

**Decision**: Rejected - would break backward compatibility

## References

- Jira Issue: [CORS-4448](https://redhat.atlassian.net/issues/CORS-4448)
- GCP Documentation: [Regional External Load Balancers](https://cloud.google.com/load-balancing/docs/tcp#regional)
- Existing Code: `cloud/services/compute/loadbalancers/reconcile.go`
- Similar Pattern: Regional Internal LB implementation in same file
