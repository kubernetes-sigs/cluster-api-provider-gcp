---
title: Regional Load Balancer Support
authors:
- "@barbacbd"
- "@DvdChe"
reviewers: []
creation-date: 2026-06-19
last-updated: 2026-10-14
status: implementable
see-also: []
replaces: []
superseded-by: []
---

# Regional Load Balancer Support

## Table of Contents

- [Regional Load Balancer Support](#regional-load-balancer-support)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
    - [API Changes](#api-changes)
    - [Feature Gate](#feature-gate)
    - [Admission Validation](#admission-validation)
    - [Reconciliation](#reconciliation)
    - [Deletion](#deletion)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
  - [Implementation History](#implementation-history)

## Glossary

- **Global External Proxy Load Balancer** - the load balancer CAPG creates by default for the Kubernetes API server. All of its components are global GCP resources.
- **Regional External Proxy Load Balancer** - a proxy load balancer whose components are all regional GCP resources. It requires a [proxy-only subnet](https://cloud.google.com/load-balancing/docs/proxy-only-subnets) in the region.
- **Regional Internal Passthrough Load Balancer** - the internal load balancer CAPG already creates for the `Internal` and `InternalExternal` types.
- **GCD** - Google Cloud Distributed / sovereign cloud environments, which only support regional resources.

## Summary

CAPG creates a Global External Proxy Load Balancer for the API server, or a Regional Internal Passthrough Load Balancer, or both. Environments such as GCD do not support global load balancing resources, so an externally reachable cluster cannot be created there today.

This proposal adds two load balancer types, `RegionalExternal` and `RegionalInternalExternal`, which create a Regional External Proxy Load Balancer, optionally alongside the existing internal one. It also adds an `externalLoadBalancerConfig` field to set the name and IP address of the external load balancer. Both are alpha and guarded by the `RegionalLoadBalancer` feature gate, disabled by default.

## Motivation

`createExternalLoadBalancer` builds every component of the external load balancer (health check, backend service, target TCP proxy, address and forwarding rule) with `meta.GlobalKey()`. These global resources cannot be created in GCD, which only accepts regional ones.

### Goals

- Provide an externally reachable API server endpoint built only from regional resources.
- Support a regional external load balancer together with the existing internal passthrough load balancer.
- Allow users to choose the name and static IP address of the external load balancer.
- Keep the behavior of the existing `External`, `Internal` and `InternalExternal` types unchanged.
- Ship the feature disabled by default, so that it can be enabled explicitly while it matures.

### Non-Goals/Future Work

- Migrating an existing cluster from a global to a regional load balancer. `spec.loadBalancer` is immutable; a new cluster has to be created.
- Creating or managing the proxy-only subnet outside of `spec.network.subnets`.
- Detecting GCD environments automatically (see [Alternatives](#alternatives)).

## Proposal

### User Stories

- As a cluster operator deploying to GCD, I want CAPG to create an external API server endpoint using only regional resources, so that I can create clusters there.
- As a cluster operator, I want both an external and an internal regional endpoint, so that in-VPC clients do not go through the external load balancer.
- As a cluster operator, I want to pin the external load balancer to a static IP address that I reserved beforehand, so that DNS records and firewall rules outside the cluster stay valid.

### API Changes

Two values are added to `LoadBalancerType` in `api/v1beta1/types.go`:

| Type | Load balancers created |
|------|------------------------|
| `RegionalExternal` | Regional External Proxy Load Balancer |
| `RegionalInternalExternal` | Regional External Proxy Load Balancer and Regional Internal Passthrough Load Balancer |

`LoadBalancerType` also gets a kubebuilder `Enum` marker so that unknown values are rejected by the CRD schema.

An optional `externalLoadBalancerConfig` field is added to `LoadBalancerSpec`. It configures the external load balancer of every type that has one (`External`, `InternalExternal`, `RegionalExternal`, `RegionalInternalExternal`); the scope, global or regional, is decided by `loadBalancerType` alone. It uses a dedicated type rather than the existing `LoadBalancer` type, so that internal-only fields such as `subnet` and `internalAccess` do not appear in the external configuration:

```go
type ExternalLoadBalancer struct {
    // Name is the name of the Load Balancer. The load balancer resources are
    // named "<cluster-name>-<name>". If not set, "apiserver" is used.
    // +optional
    Name *string `json:"name,omitempty"`

    // IPAddress is the static IP address to use for the external Load Balancer.
    // For a Global External Load Balancer this must be a global external
    // address; for a Regional External Load Balancer it must be a valid
    // external IP address in the configured region.
    // +optional
    IPAddress *string `json:"ipAddress,omitempty"`
}
```

Example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
spec:
  region: us-central1
  network:
    subnets:
    - name: control-plane-subnet
      cidrBlock: 10.0.0.0/24
    - name: proxy-only-subnet
      cidrBlock: 10.1.0.0/26
      purpose: REGIONAL_MANAGED_PROXY
  loadBalancer:
    loadBalancerType: RegionalInternalExternal
    externalLoadBalancerConfig:
      name: custom-external-lb
      ipAddress: 203.0.113.10
    internalLoadBalancer:
      name: custom-internal-lb
      subnet: control-plane-subnet
```

### Feature Gate

The new load balancer types and `externalLoadBalancerConfig` are guarded by the alpha `RegionalLoadBalancer` feature gate, disabled by default. It is set with the `EXP_CAPG_REGIONAL_LOAD_BALANCER` variable, like the existing `GKE` gate:

```shell
export EXP_CAPG_REGIONAL_LOAD_BALANCER=true
clusterctl init --infrastructure gcp
```

While the gate is disabled:

- `GCPCluster` creation is rejected if it uses `RegionalExternal`, `RegionalInternalExternal` or `externalLoadBalancerConfig`.
- The load balancer reconciler refuses such a spec, which covers clients that bypass admission.
- Updates and deletion are not gated. `spec.loadBalancer` is immutable, and gating updates would block metadata changes, such as finalizer removal, on clusters created while the gate was enabled. Those clusters can therefore still be deleted after the gate is disabled.

### Admission Validation

The `GCPCluster` webhook validates `spec.loadBalancer` on create and update. The validation lives in the webhook package rather than in the API types, which only import the packages allowed by the project's `depguard` rules. It:

- rejects unknown `loadBalancerType` values, for clients that bypass the CRD schema;
- returns a warning when `externalLoadBalancerConfig` is set with `loadBalancerType: Internal`, since there is no external load balancer to configure.

### Reconciliation

All components of the regional external load balancer use `meta.RegionalKey(name, region)`:

```text
            Clients
               |
               v
  Regional Forwarding Rule      scheme EXTERNAL_MANAGED, address: Regional Address
               |
               v
  Regional Target TCP Proxy
               |
               v
  Regional Backend Service      scheme EXTERNAL_MANAGED, mode CONNECTION, maxConnections 1000
               |
               v
  Regional Health Check         HTTPS on port 6443
               |
               v
  Instance groups (zones of the region)

  Regional Address              type EXTERNAL, optionally externalLoadBalancerConfig.ipAddress
```

`Reconcile` dispatches on the load balancer type through small predicates: `shouldCreateGlobalExternalLoadBalancer` (`External`, `InternalExternal`), `shouldCreateInternalLoadBalancer` (`Internal`, `InternalExternal`, `RegionalInternalExternal`) and `isRegionalExternalLoadBalancer` (`RegionalExternal`, `RegionalInternalExternal`). The existing types keep their original code paths.

`createRegionalExternalLoadBalancer` creates the components in dependency order: health check, backend service, target TCP proxy, address, then forwarding rule. It relies on these helpers in `cloud/services/compute/loadbalancers/reconcile.go`:

- `ensureRegionalProxyOnlySubnet` checks first that `spec.network.subnets` contains a subnet with `purpose: REGIONAL_MANAGED_PROXY` in the cluster region. If not, reconciliation fails with an error that explains how to add one, instead of an opaque GCP error when the forwarding rule is created.
- `createOrGetRegionalBackendService` is shared with the internal passthrough load balancer and takes the load balancing scheme explicitly. With `RegionalInternalExternal` it is called once with `EXTERNAL_MANAGED` and once with `INTERNAL`, so the two backends cannot be confused. `maxConnections` is only set for `EXTERNAL_MANAGED`, because GCP rejects it on `INTERNAL` backend services.
- `createOrGetRegionalTargetTCPProxy`, `createOrGetRegionalAddress` and `createOrGetRegionalExternalForwardingRule` create the remaining regional components. The service gets a new `regionaltargettcpproxies` client for the regional target TCP proxies.

When `externalLoadBalancerConfig.ipAddress` is set, both `createOrGetAddress` (global) and `createOrGetRegionalAddress` (regional) reserve that address, as `createOrGetInternalAddress` already did for the internal load balancer.

### Deletion

Components are deleted in reverse dependency order: forwarding rule, address, target TCP proxy, backend service, health check. Each load balancer teardown runs through `runDeleteSteps`, which attempts every step even when one fails and returns the aggregated errors for retry. A transient error on one component therefore does not prevent the others from being reclaimed, and cannot leave cluster deletion permanently blocked.

### Risks and Mitigations

- **API compatibility**: the new types and field are opt-in, `External` remains the default, and the feature gate keeps them unavailable unless explicitly enabled.
- **Missing proxy-only subnet**: a Regional External Proxy Load Balancer cannot be created without one. The reconciler checks for it before creating any regional external component and reports an actionable error.
- **GCP API support**: regional target TCP proxies must be available in the target environment. They are exposed by the `k8s-cloud-provider` library through `RegionTargetTcpProxies`.
- **Resource naming**: regional and global resources live under different resource paths, so the default `apiserver` name does not collide between the two scopes.
- **Migration**: moving an existing cluster from a global to a regional load balancer is out of scope; `spec.loadBalancer` is immutable.

## Alternatives

- **Detect GCD automatically** and create regional load balancers there. Rejected: less explicit, harder to test, and it may surprise users.
- **Add a boolean such as `useRegionalResources`**. Rejected: less descriptive than dedicated types, and harder to extend to mixed external/internal setups.
- **Reuse the `External` type with a scope setting**. Rejected: it changes the meaning of an existing value and makes the configuration less clear.

## Upgrade Strategy

Existing clusters are not affected: they keep their load balancer type and code path, and the feature gate is disabled by default. Using the new types requires enabling the gate and creating a new cluster.

## Additional Details

### Test Plan

- Unit tests for the dispatch predicates, `createBackends`, load balancer naming, the regional components (address, target TCP proxy, forwarding rule), the proxy-only subnet check, `runDeleteSteps` and the feature gate check in `cloud/services/compute/loadbalancers`.
- Backward compatibility tests verifying that the existing load balancer types keep their original code paths.
- Unit tests for the load balancer validation and for the `GCPCluster` webhook, including the behavior of the feature gate on create and on update.
- An e2e test that creates a cluster with a `RegionalExternal` load balancer (`cluster-template-ci-with-regional-external-lb.yaml`), with the feature gate enabled in the CI configuration.

## Implementation History

- [x] 2026-06-19: Initial proposal and implementation of the `RegionalExternal` and `RegionalInternalExternal` types.
- [x] 2026-07-03: Admission validation, proxy-only subnet check and resilient teardown, following review.
- [x] 2026-10-14: `RegionalLoadBalancer` feature gate; proposal updated to match the implementation.
