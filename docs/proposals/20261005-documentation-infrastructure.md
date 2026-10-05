---
title: Documentation Infrastructure for CAPG
authors:
  - "@jonathanrainer"
reviewers: []
creation-date: 2026-10-05
last-updated: 2026-10-05
status: provisional
see-also: []
replaces: []
superseded-by: []
---

# Documentation Infrastructure for CAPG

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Glossary](#glossary)
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals/Future Work](#non-goalsfuture-work)
- [Proposal](#proposal)
    - [CRD Reference Documentation](#crd-reference-documentation)
    - [Publishing Documentation Per Release](#publishing-documentation-per-release)
    - [Page Guidance](#page-guidance)
  - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Alternatives](#alternatives)
- [Implementation History](#implementation-history)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Glossary
- **[Diátaxis](https://diataxis.fr/)** — a documentation framework that sorts pages into four types: tutorials, how-to guides, reference 
and explanation.

## Summary
At present the documentation we have for CAPG is good but uneven, some features are documented extensively, especially 
some of the newer ones, while other older and more foundational features are documented less so. This proposal proposes
some changes to our documentation infrastructure to make the process of improving our documentation easier, namely:

1. A generated API reference for all the CRDs we maintain
2. Separate book publications for each release
3. Guidance for developers as to where to place new documentation, following the Diátaxis principles.

## Motivation
As mentioned above, our story around documentation within the repo is good in places and in others less so. This is 
entirely to be expected of a project like this and so requires occasional wholesale audits of the documentation to 
ensure it's in-line with expectations, can easily be understood by new developers etc.

To this end we should apply the Diátaxis principles to help us sharpen up our documentation, not a wholesale 
re-organisation, but re-shaping what we currently have and plugging gaps where possible.

However, there are some glaring omissions from our documentation, particularly around an API reference for the CRDs, 
amongst other things, that we need to sort first before we can do the above and that is what this proposal focusses on.
Namely, we should follow mainstream CAPI in two respects, first we should produce a generated form of API reference for 
our CRDs (as CAPI did in [this PR](https://github.com/kubernetes-sigs/cluster-api/pull/13419)) and we should 
publish all our documentation in branches that correspond to the release branches we have so that users are always 
looking at documentation that is relevant to the release they're using.

By way of example, `managed/network-config.md` is currently live despite the fact that it's slated for a 
future release.

As a final point we should also produce some documentation about where developers should place new documentation, which
should help "staunch the bleeding" and then we can focus on the documentation audit after the fact.

### Goals

- Get in place a page in our documentation that is a reference for our CRD APIs (using `crd-ref-docs`)
- Publish documentation per release branch, with the main URL serving the latest release
- Produce guidance for developers in our developer docs about where new documentation should be put

### Non-Goals/Future Work

- Do a complete documentation audit as part of this work
- Re-organise what documentation we have
- Strictly adhere to Diátaxis principles throughout 

## Proposal

Taking each piece in turn this proposal breaks down in the following way

#### CRD Reference Documentation

To generate the reference documentation we should use [`elastic/crd-ref-docs`](https://github.com/elastic/crd-ref-docs) with its Markdown renderer, as CAPI
does, to produce a new page within the "book" documentation. This is chosen over the other tools used by CAPA, CAPZ and 
CAPO on the grounds that it produces the best quality output (in initial testing) and is most in keeping with the 
documentation we already publish.

Concretely we'd need to:
- Pin the tool in `hack/tools`.
- Add `make generate-crd-docs`, writing `docs/book/src/reference/api-reference.md`, and include it in `make generate`.
- Add sensible configuration to `hack/crd-ref-docs-config.yaml`, this can help us to ignore some generated types and 
link out to CAPI docs etc.

This will produce a single page covering `api/v1beta1`, `exp/api/v1beta1` and `exp/bootstrap/gke/api/v1beta1`. 
Self-managed and GKE types share the `infrastructure.cluster.x-k8s.io` group, so links between them 
(e.g. `GCPManagedClusterSpec` → `NetworkSpec`) resolve within the page. A trial run produced 113 types with no broken 
links within the page.

#### Publishing Documentation Per Release

At present in CAPI they publish documentation as follows:

| Host                                  | Serves                |
|---------------------------------------|-----------------------|
| `cluster-api.sigs.k8s.io`             | latest release branch |
| `release-1-N.cluster-api.sigs.k8s.io` | `release-1.N` branch  |
| `main.cluster-api.sigs.k8s.io`        | `main`                |

We should take the same approach, updating our release process to raise a PR to add the required DNS record, as CAPI 
does. This gets us away from the position we're in right now, where documentation updates show up outside a release, 
which could make people think features are usable before they actually are.

Concretely we'd need to:

1. **Netlify:** enable branch deploys for `release-*` branches on the `kubernetes-sigs-cluster-api-gcp` site. 
`main` is already deployed at `main--kubernetes-sigs-cluster-api-gcp.netlify.app`.
2. **DNS (kubernetes/k8s.io):** add `release-1-N.cluster-api-gcp.sigs` and `main.cluster-api-gcp.sigs` CNAMEs to the 
matching branch deploys, and point `cluster-api-gcp.sigs` at the latest release branch deploy instead of the current 
production site. CAPI's records in `dns/zone-configs/k8s.io._0_base.yaml` are the model.
3. **Book:** list the versioned books on the introduction page, and add a note to the `main` build that it may describe 
unreleased features.
4. **Release process:** add steps to `developers/releasing.md`: when a release branch is cut, raise the k8s.io DNS PR 
for the new subdomain and move the main domain to it. This is a manual step but it would never block the release and
again is in line with CAPI.

#### Page Guidance

The final step is some guidance on addition of documentation pages, we should add documentation there that describes:

- the four Diátaxis page types, and that each page should be one of them
- placement: GKE-only pages in `managed/`, cross-cutting pages in `topics/`
- that how-to pages link to the API reference rather than repeating field lists
- what a feature PR is expected to include: a how-to page when the feature involves a choice or a multi-step setup, 
otherwise the generated reference is enough

### Implementation Details/Notes/Constraints

As noted above, initial experiments with `ahmetb/gen-crd-api-reference-docs` and `theunrepentantgeek/crddoc` did not
produce great results, so I considered that aligning with CAPI was a better bet.

Considering that we haven't published documentation for our releases like this up to this point, I propose that we
_attempt_ to backport this process to 1.11 and versions north of that. This can then be in place for any future versions
after the fact.

### Risks and Mitigations

The risks of this change are very small and most relate to humans forgetting steps in a process or blocking releases on
external reviews. This should be solved by updates to `releasing.md`.

## Alternatives

We could just add a banner, which is cheap but that doesn't tell users the specific fields their release 
has. We could also link out to `doc.crds.dev` but then that severs the link between the code and the documentation we 
produce. In addition CAPI moved away from that approach as this 
[issue](https://github.com/kubernetes-sigs/cluster-api/issues/13273) documents. Finally, we could just manually annotate each 
CRD field with "added in vX.Y" but that would get very onerous very quickly and could easily be forgotten.

In terms of tool comparison the preliminary results are listed below:

| Tool                                | Used by    | Result                                                                                                  |
|-------------------------------------|------------|---------------------------------------------------------------------------------------------------------|
| `elastic/crd-ref-docs`              | CAPI       | Markdown; inlines embedded structs; renders defaults and validation from kubebuilder markers            |
| `ahmetb/gen-crd-api-reference-docs` | CAPA, CAPO | Raw HTML; embedded structs shown only as a link; needs a `doc.go` with `+groupName` in each API package |
| `theunrepentantgeek/crddoc`         | CAPZ       | Markdown; embedded structs shown only as a link; no defaults or validation                              |

## Implementation History

- [ ] 2026-10-05: Opened Proposal PR
