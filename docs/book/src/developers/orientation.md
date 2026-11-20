# Repository Orientation

The CAPG repository contains a number of sub-folders which allow us to organise different pieces of code and documentation
into different areas. We largely follow Go conventions around project layout so it should be easy to navigate if you're
familiar with Go projects.

## Modules and Dependencies

This repository uses [Go Modules](https://github.com/golang/go/wiki/Modules) to track vendor dependencies.

To pin a new dependency:

- Run `go get <repository>@<version>`
- (Optional) Add a replace statement in `go.mod`

Makefile targets and scripts are offered to work with go modules:

- `make verify-modules` checks whether go modules are out of date.
- `make modules` runs `go mod tidy` to ensure proper vendoring.
- `hack/ensure-go.sh` checks that the Go version and environment variables are properly set.

## CI Jobs

Our CI configuration is split between jobs that run in Prow, and are configured in
[kubernetes/test-infra](https://github.com/kubernetes/test-infra/tree/master/config/jobs/kubernetes-sigs/cluster-api-provider-gcp), and GitHub Actions which are configured in the
[`.github` directory](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/.github/workflows).

### Pull Request Checks

To see these tests in one place you can look at [Testgrid](https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp).

#### Presubmits - Prow

In the table below … refers to `cluster-api-provider-gcp`.

| Job | When | Runs | What it does |
|---|---|---|---|
| `pull-…-test` | always, required | `scripts/ci-test.sh` | `make test-junit`: unit and envtest integration tests, junit report |
| `pull-…-build` | always, required | `scripts/ci-build.sh` | `make binaries`: compiles the manager |
| `pull-…-make` | always, required | `runner.sh scripts/ci-make.sh` | `make docker-build`: builds the controller image (docker-in-docker) |
| `pull-…-verify` | always, required | `runner.sh make verify` | generated files, modules, conversions, boilerplate, shellcheck, Tiltfile, go directive, doctoc |
| `pull-…-e2e-test` | always, required | `ci-e2e.sh --flavor=unmanaged`, `GINKGO_FOCUS="Workload cluster creation"` | self-managed workload clusters on real GCP (Boskos project) |
| `pull-…-e2e-test-gke` | always, required | `ci-e2e.sh --flavor=gke`, `GINKGO_FOCUS="GKE workload cluster"` | managed (GKE) workload clusters |
| `pull-…-apidiff` | always, optional | `runner.sh scripts/ci-apidiff.sh` | `make apidiff` against `PULL_BASE_SHA` |
| `pull-…-conformance` | manual | `ci-conformance.sh` | Kubernetes conformance on a workload cluster |
| `pull-…-conformance-ci-artifacts` | manual | `ci-conformance.sh`, `E2E_ARGS=-kubetest.use-ci-artifacts` | same, with Kubernetes CI artifacts |
| `pull-…-capi-e2e` | manual | `ci-e2e.sh --flavor=unmanaged`, focus "Cluster API E2E tests", skip `[K8s-Upgrade]` and API Version Upgrade | upstream CAPI e2e suite |
| `pull-…-e2e-workload-upgrade` | manual | `ci-e2e.sh --flavor=upgrade`, focus `[K8s-Upgrade]` | Kubernetes upgrade of a workload cluster |
| `pull-…-coverage` | manual | `ci-test-coverage.sh` plus gopherage | coverage report|

To run any of the manual jobs comment `/test <full job name>` on a PR.

#### Presubmits - GitHub Actions

| Workflow | Trigger | Does |
|---|---|---|
| Lint (`lint.yml`) | every PR | golangci-lint |
| Codespell | every PR | spelling over the tree, `.codespellignore` |
| Link-check | PRs touching `docs/**`; push to `main` touching `docs/**` | `markdown-link-check` with `.markdownlinkcheck.json` |
| PR approve GH Workflows | `pull_request_target` on PRs labelled `ok-to-test` | approves the pending Actions runs for them |

### Periodics

Some CAPG jobs run periodically, specifically we run `build`, `test` and `conformance` checks every 12h on our `main`
branch and any actively maintained release branches.
