# Contributing Guidelines
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Contributor License Agreements](#contributor-license-agreements)
- [Finding Things That Need Help](#finding-things-that-need-help)
- [Contributing a Patch](#contributing-a-patch)
- [Proposal Process](#proposal-process)
- [Backporting a Patch](#backporting-a-patch)
- [Merge Approval](#merge-approval)
- [Google Doc Viewing Permissions](#google-doc-viewing-permissions)
- [Issue and Pull Request Management](#issue-and-pull-request-management)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Read the following guide if you're interested in contributing to cluster-api-provider-gcp (CAPG).

## Contributor License Agreements

We'd love to accept your patches! Before we can take them, we have to jump a couple of legal hurdles.

Please fill out either the individual or corporate Contributor License Agreement (CLA). More information about the CLA
and instructions for signing it [can be found here](https://github.com/kubernetes/community/blob/master/CLA.md).

***NOTE***: Only original source code from you and other people that have signed the CLA can be accepted into the
repository.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that
should not need deep knowledge of the system. [Have a look and see if anything sounds
interesting](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).
Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that
come up, including gaps in documentation!

## Contributing a Patch

For AI-assisted contributions, follow the project's [AI Usage
Policy](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/AI_POLICY.md).

1. If you haven't already done so, sign a Contributor License Agreement (see details above).
2. Fork the desired repo, develop and test your code changes.
    - There are Makefile targets available to help with running our automated tests including
      - `make lint` - Run our Code Linters
      - `make test` - See below
      - `make verify` - Run a variety of static checks over code and non-code files
      - `make generate` - Ensure that generated files are up to date with their sources
    - `make test` executes the project's unit and integration tests. These tests need no Kubernetes cluster or GCP
      access. Some of them (the `envtest` suites) start a local API server and `etcd`, whose binaries `make test`
      downloads on first use.
    - We have standardized on a number of frameworks for testing, so when adding new tests:
      - Unit Tests: These are plain `go test` tests with `gomega` accessed via `NewWithT(t)`. This is in preference to
        `testify`, which will cause linting to fail if used.
        - When using `g := NewWithT(t)` you should always use the 'nearest' `t`, rather than anything global. Definitely
          do not use the package-level `Expect`.
      - Tests that require `envtest`: These are written with Ginkgo and are often useful for testing CRD and CEL
        validation
      - End-To-End Tests: These are written with Ginkgo.
3. Submit a pull request.
   1. All PRs must be labeled with one of the following kinds
      - `/kind feature` for PRs related to adding new features/tests
      - `/kind bug` for PRs related to bug fixes and patches
      - `/kind api-change` for PRs related to adding, removing, or otherwise changing an API
      - `/kind cleanup` for PRs related to code refactoring and cleanup
      - `/kind deprecation` for PRs related to a feature/enhancement marked for deprecation
      - `/kind design` for PRs related to design proposals
      - `/kind documentation` for PRs related to documentation
      - `/kind failing-test` for PRs related to a consistently or frequently failing test
      - `/kind flake` for PRs related to a flaky test
      - `/kind other` for PRs related to updating dependencies, minor changes or other
   2. If the PR requires additional action from users switching to a new release, include the string "action required"
      in the PR release-notes.
   3. All behavior changes must be covered by tests. Always use the smallest test layer possible, especially as E2E
      tests create billable GCP resources, see our [E2E
      documentation](https://cluster-api-gcp.sigs.k8s.io/developers/e2e.html) for more details.
   4. All new features should come with user documentation.
4. Once the PR has been reviewed and is ready to be merged, commits should be
   [squashed](https://github.com/kubernetes/community/blob/master/contributors/guide/github-workflow.md#squash-commits).
   - Ensure commit message(s) are meaningful and message history is readable.
   - Ensure commit messages adhere to [Conventional Commits](https://www.conventionalcommits.org/).

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer
docs](https://github.com/kubernetes/community/tree/master/contributors/devel). Expect reviewers to request that you
avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

## Proposal Process

Larger changes, such as new features, API changes or changes to how the project is built and released, should start as a
proposal. CAPG follows the [Cluster API proposal
process](https://github.com/kubernetes-sigs/cluster-api/blob/main/CONTRIBUTING.md#proposal-process-caep):

- Proposals SHOULD be introduced at the [CAPG office
  hours](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/README.md#office-hours) or in
  [#cluster-api-gcp](https://kubernetes.slack.com/archives/C01D1RFEN9G) on Slack before a PR is opened.
- Proposals are submitted as a PR to
  [docs/proposals](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/docs/proposals), named
  `YYYYMMDD-my-title.md` and based on the [Cluster API proposal
  template](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/YYYYMMDD-template.md). Label the PR
  `/kind design`.
- Proposals MUST be merged and in `implementable` state before the work they describe is merged.
- Once implemented, proposals are moved to the [Proposal
  Archive](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/docs/proposals/archived) with their
  status set to `implemented`.

## Backporting a Patch

CAPG ships older versions through `release-X.X` branches, usually backports are reserved to critical bug-fixes. Check
with the maintainers if you need a fix backporting.

## Merge Approval

CAPG maintainers may add "LGTM" (Looks Good To Me) or an equivalent comment to indicate that a PR is acceptable. Any
change requires at least one LGTM.  No pull requests can be merged until at least one CAPG maintainer signs off with an
LGTM.

## Google Doc Viewing Permissions

To gain viewing permissions to google docs in this project, please join either the
[kubernetes-dev](https://groups.google.com/a/kubernetes.io/g/dev) or
[kubernetes-sig-cluster-lifecycle](https://groups.google.com/a/kubernetes.io/g/sig-cluster-lifecycle) google group.

## Issue and Pull Request Management

Anyone may comment on issues and submit reviews for pull requests. However, in order to be assigned an issue or pull
request, you must be a member of the [Kubernetes SIGs](https://github.com/kubernetes-sigs) GitHub organization.

If you are a Kubernetes GitHub organization member, you are eligible for membership in the Kubernetes SIGs GitHub
organization and can request membership by [opening an
issue](https://github.com/kubernetes/org/issues/new?template=membership.md&title=REQUEST%3A%20New%20membership%20for%20%3Cyour-GH-handle%3E)
against the kubernetes/org repo.

However, if you are a member of any of the related Kubernetes GitHub organizations but not of the Kubernetes org, you
will need explicit sponsorship for your membership request. You can read more about Kubernetes membership and
sponsorship [here](https://github.com/kubernetes/community/blob/master/community-membership.md).

CAPG maintainers can assign you an issue or pull request by leaving a `/assign <your Github ID>` comment on the issue or
pull request.
