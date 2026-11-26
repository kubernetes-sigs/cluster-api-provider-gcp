---
title: Container Network Interface (CNI) Delivery & Support Options
authors:
  - "@jonathanrainer"
reviewers: []
creation-date: 2026-11-26
last-updated: 2026-11-26
status: provisional
see-also:
  - https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues/1795
  - https://github.com/kubernetes-sigs/cluster-api-provider-gcp/pull/1794
replaces: []
superseded-by: []
---

# Container Network Interface (CNI) Delivery & Support Options

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Glossary](#glossary)
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
- [Proposal](#proposal)
  - [Delivery of CNIs](#delivery-of-cnis)
  - [Per-CNI Considerations/Specializations](#per-cni-considerationsspecializations)
    - [Calico](#calico)
    - [Cilium](#cilium)
    - [Flannel](#flannel)
  - [Template Matrix & Construction](#template-matrix--construction)
  - [End-To-End Testing & Developer Experience](#end-to-end-testing--developer-experience)
  - [Documentation](#documentation)
  - [User Stories](#user-stories)
  - [Implementation Details](#implementation-details)
    - [Leftover Resources](#leftover-resources)
    - [Timeouts On Teardown](#timeouts-on-teardown)
    - [Version Pinning](#version-pinning)
    - [Upgrading a CNI](#upgrading-a-cni)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Alternatives](#alternatives)
- [Upgrade Strategy](#upgrade-strategy)
- [Conclusion](#conclusion)
- [Implementation History](#implementation-history)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Glossary

**CAAPH** - The Cluster API Addon Provider for Helm, it allows you to install Helm Charts into Cluster API provisioned
clusters, without needing a separate out-of-band process.

**HelmChartProxy** - The CustomResource that CAAPH provides to represent the Helm Chart that you want installed into the
provisioned cluster.

**ClusterResourceSet (CRS)** - A CustomResource that defines a set of Kubernetes resources you want applied to the
provisioned cluster.

## Summary

CAPG's support for CNIs at present is not very well documented and our tested mechanism is different to what we
recommend to users. CAPG should differentiate between supported CNIs and recommended CNIs while simplifying our delivery
of the CNI itself. The ultimate goal should be one "batteries-included" template per supported CNI, exercised in our E2E
tests, as well as continuing to support a template without one so users are not blocked if they want to use a custom
one. These should further be delivered by [CAAPH (Cluster API Addon Provider for
Helm)](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm) to reduce the need to embed constantly
changing manifests inside ClusterResourceSets (CRSs).

This makes it much easier for people to onboard with a working configuration, while leaving power-users or users with
specific requirements free to keep any custom work they already have.

## Motivation

As stated above, CAPG currently "supports" Calico as our CNI of choice, as it's the one we end-to-end test against.
However, the only documentation we provide is for Flannel, which is out of date. Further, our E2E tests install Calico
differently from anything a user can do, and limitations around `CNI_RESOURCES` and the size of the necessary ConfigMap
that you can store inside a CRS means Calico cannot be installed at version 3.32 or greater.

Not only that but other documentation is out of sync as well, particularly the work on Alias IP routing, which doesn't
specify which CNI or mode it applies to.

All of this leads to a very complicated state of affairs for users, and since the very first line of our documentation
around CNIs is:

> By default, no CNI plugin is installed when a self-managed cluster is provisioned.

We're expecting a huge amount from users who just want to get started quickly with a CNI that works. Further it's very
confusing as to what is actually supported due to the state of the documentation and the divergence between that and
what we test etc.

### Goals

- Clearly define which CNIs are supported by CAPG and to what level.
- Create a set of templates that work and are tested with each of our supported CNIs.
- Produce documentation that allows users to make an informed choice.
- Re-orientate our E2E tests to ensure each CNI is exercised.

### Non-Goals/Future Work

- Further template changes (CCM delivery etc.)
- IPv6/dual-stack support
- Guaranteeing or documenting in-place CNI upgrades
- How this interfaces with support for Flatcar images (see
  [1791](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/pull/1791))
- Moving towards a single set of templates we test with only additional ones provided for E2E testing (this will be
  covered in a future proposal)

## Proposal

As stated above, our first port of call should be to define which CNIs we support and to what level, to this end I
propose the following:

|CNI   |Level of Support   |
|---|---|
|[Calico](https://docs.tigera.io/calico/latest/about/)|Recommended   |
|[Cilium](https://cilium.io/)|Supported   |
|[Flannel](https://github.com/flannel-io/flannel)     |Supported            |

Where "Supported" means we provide a template for the CNI and exercise a smoke test in our E2E tests. Meanwhile,
"Recommended" means that this is where we want users to start, and it's what we use by default in our testing, unless we
have a good reason not to. In this formulation "Recommended" is a stronger guarantee than "Supported" and this might
well be a distinction we want to take into other areas of the provider.

### Delivery of CNIs

At present CNI delivery is driven by the `CNI_RESOURCES` environment variables in the `cluster-template-topology.yaml`
template. As mentioned above this has a number of problems, including the fact CRSs rely on storing resources inside a
ConfigMap to then apply. This places a hard ~1Mi limit on the amount of resource you can apply, which when dealing with
complicated CustomResourceDefinitions (CRDs) you can often exceed.

Instead of that we should lean in to some of the new work being done to support "Addons" in the context of Cluster API,
particularly [CAAPH](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm). CAAPH lets us define Helm
Charts to be installed instead of raw cluster resources as CRSs do which means we can simplify the templates a lot and
gain advantage from not maintaining custom ways of installing CNIs. This is achieved by the use of `HelmChartProxy`
CustomResources, of which there would be one per cluster. This would mean that the installation command for CAPG would
become `clusterctl init --infrastructure gcp --addon helm` when CAAPH is required. This has been tested with CAPG
`1.13.1`, CAAPH `0.6.6` and CAPI `1.14.x` and these worked together without issue, however we should make sure to
publish supported ranges so this continues to be the case.

The testing mentioned above ran 4 GCP Clusters and around 15 CAPD runs using Cilium 1.20.5, Flannel v0.28.11 and Calico
v3.33.1. As such all of the proposed shapes of this proposal have had installation and connectivity tested in GCP.
Further testing should be performed as we implement the proposal.

### Per-CNI Considerations/Specializations

As each CNI is different we do need to make some affordances for each, those are explained below:

#### Calico

In recent times Calico has moved away from directly installing manifests to being controlled by an Operator and I
propose that we lean into that, by using a `HelmChartProxy` resource pointing at the [Tigera
Operator](https://artifacthub.io/packages/helm/projectcalico/tigera-operator), and a small CRS that holds the
`Installation` CustomResource. We will need to set `bgp: Enabled`, and `ipPools: [{cidr, encapsulation: IPIP}]` to match
our existing `calico.yaml` and also configure the chart to turn off `APIServer`, `Goldmane` and `Whisker` resources by
default as they are not useful for our use-case. We should add a templated variable however that allows users to turn
these on if they would like.

#### Cilium

Cilium requires little specialization, we can use the [standard chart](https://github.com/cilium/charts) we simply need
to configure `ipam.mode: cluster-pool` when it is installed, which is part of the `HelmChartProxy`.

#### Flannel

Flannel is slightly more complicated as it requires the CCM's allocator to be `RangeAllocator` when by default it's set
to `CloudAllocator`, this requires us to ensure that we add a template variable so this can be varied. This happens
because Flannel relies on the `podCIDR` being set on the node whereas KCM keeps `allocate-node-cidrs: false`. In
addition Flannel relies on `bridge` being present in the VM image which is not a worry for us as it's supplied by the
`kubernetes-cni` package that we already install.

Other than that we can use their standard [Helm Chart](https://flannel-io.github.io/flannel/).

### Template Matrix & Construction

So, with all of the above in mind, how would we deliver this to users, and within the repo itself? We should follow the
example of CAPZ, where each topology "flavour" gets variants. In our case we'd implement this with Kustomize, so where
at present we have `cluster-template.yaml`, `cluster-template-clusterclass.yaml` and `cluster-template-topology.yaml`.
These would each gain 3 additional variants, for example `cluster-template-calico.yaml`, `cluster-template-flannel.yaml`
etc. This means that our users who depend on the existing templates do not see any change, but we can provide new
options that only differ in their CNI and do not drift from the standard template as it changes.

This will mean that for most of the commands where we use `clusterctl`, particularly in tests, we'll have to add
`--flavor=calico` otherwise we'll get a cluster with no CNI. This would mean that `cluster-template.yaml` would then
become the template to use if you want to decide on your own CNI, outside our supported options, and
`cluster-template-calico.yaml` would be our recommended default. Annoyingly, `clusterctl` doesn't let you specify a
separate default to `cluster-template.yaml` so this might be surprising behaviour to users, but this is being done to
minimize the risk of breaking existing installations as we roll this out.

There would also need to be some more machinery, in the form of a script to generate the templates (and probably a CI
check to ensure the templates don't fall out of sync with the base). This would also fulfil the Flannel specific
requirements around `RangeAllocator` above.

We would exclude GKE because networking is opaque to the user there. Further, we would only support this for our own
self-built images in the first instance (we can decide later if we want to extend this to Flatcar once 1791 has merged).
We would also provide the ability to users to override several values, including chart repo URLs, versions and image
registries to allow air-gapped use-cases to continue to function.

### End-To-End Testing & Developer Experience

With the machinery we've now built, we can drastically simplify our own end-to-end testing to make sure we're enforcing
the guarantees that we claim. We can remove the CRS transformation that's there and instead use the CAAPH Calico
template. We can also add specs for Cilium and Flannel smoke tests which perform some simple checks like Pod-to-Pod DNS.

We can also simplify our own advice to developers whereby the `make create-workload-cluster` command can take a template
name so it's easy for users to spin up and test against clusters with different CNIs where that's relevant.

### Documentation

We will need to update our documentation in light of the changes listed, however most of those can be removals rather
than reams of new work. We should produce a single CNI page that details how to choose one and which template to use in
light of that. We should further enhance that with sections on "bring-your-own" and tearing down a CNI, as well as
folding in the Alias-IP page that we currently have and remove references to `CNI_RESOURCES` throughout.

In the specific case of Cilium we should also make sure to document native-routing as that is one of the main reasons
Cilium is utilised in this space.

### User Stories

- As a new user of CAPG I want to get a cluster up and running with a known-good configuration quickly
- As an existing user of CAPG that has invested in my CNI setup I do not want new versions to force me to change it
- As a developer of CAPG I want to be able to quickly test against multiple CNIs without having to configure it myself
- As a maintainer of CAPG I want to ensure that any guarantees I make about support are backed by evidence

### Implementation Details

There are a few sundry issues that are not long enough to merit their own section, however we should deal with them:

#### Leftover Resources

Due to the way that CAAPH is implemented running `kubectl delete cluster` will leave behind a labelled `HelmChartProxy`
CustomResource (as well as a CRS and ConfigMap for Calico as it contains the `Installation` CustomResource). We should
make sure to document this and provide users with a way to clean this up.

Fortunately the `HelmChartProxy` is labelled with the `cluster.x-k8s.io/cluster-name` label, and we should contrive
it such that the CRS and ConfigMap also carry the same label. Then it should be as simple as providing a
`kubectl delete` command that matches on that label, but it should still be documented.

#### Timeouts On Teardown

Additionally, we need to set `options.timeout` to `2m` otherwise we get `10m` stalls when tearing down the
CNIs. This seems to only be a problem for Calico as the other CNIs tear down in a matter of seconds, however it should
be noted that this option also impacts installs and upgrades. As a result it should be user-configurable but set to `2m`
as a default.

#### Version Pinning

We should pin versions of all the supported CNIs centrally and then create a new Claude skill called `bump-cnis` that we
can run manually, as and when necessary to bump up their versions. As we'll have test coverage in our E2E tests we can
be relatively sure when we merge changes that they will be working for users.

In an ideal world we would "outsource" this work to Dependabot, but Dependabot is very bad at scanning dependencies
embedded inside YAML files so this is not an option. If in the future we decided to move to something like Renovate we
would be able to accommodate this, but that is a much broader conversation that is not immediately relevant to this
proposal.

#### Upgrading a CNI

A final detail is that now we're explicitly listing versions for the CNIs themselves we not only have to deal with how
we bump supported versions, but what happens if a user does that while the CNI is deployed.

For Calico this should be straightforward as there is an Operator reconciling the changes, but we need to test this and
make sure that CAAPH handles the change in Cilium and Flannel as well. As the non-goals state we do not wish to document
or guarantee this process, but checking that doing this does not result in a broken state is something we should test.

### Risks and Mitigations

The biggest risk with this proposal is the inclusion of CAAPH as a hard dependency for some of our work (especially in
the realm of our end-to-end tests). If the project were to be abandoned or get behind significantly this could severely
impede our ability to deliver to our users. I believe this is a risk worth taking, for multiple reasons:

1. Delivering Calico in a sensible way is hard to do without using their Helm Chart due to the CRS restrictions
   previously discussed. As such our options are either "drop Calico support" or find a way to make it work
2. CAAPH has existed for over 3 years and shows no signs of going away
3. We could/should use this as an opportunity to support another part of the CAPI ecosystem, further the more users that
   CAAPH has, the more likely future investment of time can be justified

A second smaller risk is around silent failure modes, as it's often the case with networking that failure can be hard to
detect. For example the `Installation` CR for Calico can fail silently (as we discovered during testing when we didn't
set `ipPools[].cidr` from the Cluster). Additionally, with Flannel, even though all three nodes reported ready, pods
were starting with `pod cidr not assigned` until we fixed the `RangeAllocator` issue. This is even more of a reason
for us to invest in our supported setups so that we can ensure users are getting templates that work rather than them
running the risk of misconfiguring them.

## Alternatives

We did consider whether an alternative solution to the problems above would be to simply use a baked CRS for all CNIs.
But this was rejected on the grounds this wouldn't work for Calico and would force us to maintain several very different
solutions rather than using Helm to abstract over that difference. Additionally, this made some of the CNIs require a
two-step apply process, which the Helm chart avoids. The key thing we wanted to maintain was that a user simply has to
run one `clusterctl` command to get a working cluster, rather than a multistep process.

## Upgrade Strategy

For existing users the Upgrade Strategy should be painless as we are not proposing behaviourally changing any of the
templates that currently exist. This proposal is mostly for the benefit of devs who want ephemeral versions of these
CNIs or new users who want a "batteries-included" way to get started.

## Conclusion

At present our CNI story is confused and is the product of something that has evolved rather than being designed from
the outset. This proposal allows us to formulate not only a set of supported CNIs but also the machinery to keep them
supported into the future. New users can install our templates, safe in the knowledge that their networking
configuration, so often the cause of problems, is tested and attested by CAPG before they ever apply it.

## Implementation History

- [X] 2026/11/26 - Initial version of proposal published
