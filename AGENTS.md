# Agent Development Guide

This guide describes the repository layout and development practices for
Cluster API Provider GCP (CAPG). Follow the existing code and test patterns in
the area you are changing. Also read and strictly follow the [AI_POLICY.md](AI_POLICY.md) for the
project's AI-assisted contribution expectations.
Exceptions apply for maintainers, when explicitly needed.

## Rules and Constraints

- Keep changes focused on the requested work. Do not create commits or pull
  requests unless explicitly asked.
- Use [Conventional Commits](https://www.conventionalcommits.org/) format for
  commit messages, PR titles, and release notes:
  `<type>[optional scope]: <description>` (for example, `fix: handle missing
  GCPCluster`).
- Add or update tests for behavior changes. Prefer tests alongside the code,
  following the patterns already used in that package.
- make lint and make test must be clean before handoff.
- Do not edit generated code or manifests by hand. Run `make generate` and
  include the resulting changes when generated output needs updating.
- Keep the Go version and dependencies in `go.mod` and `hack/tools/go.mod`
  unchanged unless the task requires a dependency or Go version update.
- New API fields need appropriate descriptions and validation. Core provider
  API types are in `api/v1beta1`; experimental APIs are in `exp/api/v1beta1`
  and `exp/bootstrap/gke/api/v1beta1`. Update generated code and manifests
  through the repository's generation targets.
- GKE-related experimental functionality is feature-gated. Check
  `feature/feature.go` and the relevant setup in `main.go` when changing it.

## Project Layout

| Area | Location |
| --- | --- |
| Core CAPG API types | `api/v1beta1/` |
| Experimental infrastructure APIs | `exp/api/v1beta1/` |
| Experimental GKE bootstrap API | `exp/bootstrap/gke/api/v1beta1/` |
| Core cluster and machine controllers | `controllers/` |
| Experimental controllers | `exp/controllers/` |
| GKE bootstrap controller | `exp/bootstrap/gke/controllers/` |
| Admission webhooks | `webhooks/`, `exp/webhooks/` |
| GCP service clients and reconciliation | `cloud/services/compute/`, `cloud/services/container/` |
| Shared scopes and GCP client setup | `cloud/scope/` |
| Generated CRDs, RBAC, and deployment configuration | `config/` |
| End-to-end tests | `test/e2e/` |

`main.go` wires the manager, controllers, webhooks, and feature gates. Core
resources include `GCPCluster` and `GCPMachine`; experimental GKE resources
and their controllers are under `exp/`.

## Data Flow

```text
User creates CAPI and CAPG resources
  → Manager watches resources and enqueues the relevant controller
  → Reconciler loads the owning CAPI resources and creates a scope
  → Scope and service reconcilers compare desired state with GCP resources
  → Compute Engine or GKE APIs are called to create, update, or delete resources
  → Controller updates status, conditions, events, and finalizers
  → Reconcile again when cloud operations are still in progress
```

Core infrastructure reconciliation uses `GCPCluster` and `GCPMachine` with
services under `cloud/services/compute/`. Experimental managed-cluster and
bootstrap flows use the resources under `exp/` and services under
`cloud/services/container/`.

## Development and Validation

The repository's `Makefile` is the source of truth for available targets.
Common commands are:

```bash
make test       # Unit and integration tests
make lint       # Go lint checks
make generate   # Generate Go code and Kubernetes manifests
make verify     # Verify generated output, modules, conversions, and formatting
```

End-to-end tests require a configured GCP environment and credentials. Run
`make test-e2e` only when that environment is available.

API or controller changes may require updating user documentation under
`docs/book/` and tests under the corresponding package. Keep generated CRDs,
RBAC, and other generated files in sync by running `make generate`.
