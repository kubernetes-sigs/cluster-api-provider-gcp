# Bumping Kubernetes and Cluster API

This document describes how to bump the Kubernetes and Cluster API (CAPI) versions across the project. It is primarily intended to be consumed by an AI coding agent (e.g. via `/bump-k8s-capi 1.35 1.13`), but the steps can also be followed manually.

The two arguments are the target **Kubernetes minor version** (e.g. `1.35`) and the target **CAPI minor version** (e.g. `1.13`).

## Convention

- Kubernetes module version: `v0.MINOR.PATCH` (e.g. `v0.35.4` for k8s 1.35)
- CAPI version: `v1.MINOR.PATCH` (e.g. `v1.13.2`)
- Infrastructure provider dev version: `v1.CAPI_MINOR.99` (e.g. `v1.13.99`)
- CCM version major matches the Kubernetes minor (e.g. k8s 1.35 → CCM `v35.x.y`)

## Step 1: Research

Perform these lookups before making any changes. All values determined here are referenced in later steps.

Kubernetes-version-derived variables in `test/e2e/config/gcp-ci.yaml`
(`KUBERNETES_VERSION`, `KUBERNETES_VERSION_GKE`, `CCM_VERSION`,
`KUBERNETES_VERSION_MANAGEMENT`, and the upgrade-test FROM/TO/etcd/coredns/
image variables) are **not** looked up here — they derive automatically at
CI time from `KUBERNETES_MINOR` via `hack/resolve-e2e-versions.sh`. The one
thing to check by hand before setting `KUBERNETES_MINOR` is that GKE
actually supports it yet (1f below) — that's a precondition for the bump,
not something to route around if it doesn't.

### 1a. Latest CAPI patch

```bash
gh api repos/kubernetes-sigs/cluster-api/tags --paginate -q '.[].name' | grep '^v1.MINOR\.' | head -5
```

Pick the latest stable tag. Call it `CAPI_VERSION` (e.g. `v1.13.2`).

### 1b. CAPI dependency versions

Fetch CAPI's `go.mod` to determine aligned dependency versions:

```bash
gh api "repos/kubernetes-sigs/cluster-api/contents/go.mod?ref=CAPI_VERSION" --jq '.content' | base64 -d
```

Extract:
- `sigs.k8s.io/controller-runtime` version → `CONTROLLER_RUNTIME_VER`
- `k8s.io/api` version → `K8S_MODULE_VER` (e.g. `v0.35.4`)

### 1c. setup-envtest version

Fetch CAPI's `Makefile` to align the setup-envtest CLI version:

```bash
gh api "repos/kubernetes-sigs/cluster-api/contents/Makefile?ref=CAPI_VERSION" --jq '.content' | base64 -d | grep '^SETUP_ENVTEST_VER :='
```

Call this `SETUP_ENVTEST_VER`. The controller-gen and conversion-gen versions are selected by `hack/tools/go.mod`, not copied from CAPI's `Makefile`.

### 1d. GCP k8s-cloud-provider version

```bash
gh api repos/GoogleCloudPlatform/k8s-cloud-provider/tags -q '.[].name' | head -5
```

Pick the latest matching the target k8s minor. Call it `K8S_CLOUD_PROVIDER_VER`.

### 1e. kind version

Kind follows the version selected by the root `go.mod`; do not choose a separate Makefile pin. After updating dependencies in Step 3, check the selected version with `go list -m sigs.k8s.io/kind`.

### 1f. Confirm GKE supports the target minor

```bash
gcloud container get-server-config --region=us-central1 --format=json | \
  jq -r '.channels[] | select(.channel=="REGULAR") | .validVersions[]' | \
  grep '^K8S_MINOR\.' | sort -V | tail -5
```

If nothing matches, GKE hasn't caught up to this minor yet. **Don't** bump
`KUBERNETES_MINOR` past it — wait for GKE to catch up instead. This
project bumps to the trailing edge of Kubernetes releases, not the
bleeding edge, so this should be rare; the more likely failure mode over
time is the opposite one (GKE eventually dropping support for a minor
this project sat on too long), which the same check catches just as well.

## Step 2: Review CAPI migration guide

Read the upstream CAPI migration guide for the version jump being performed:

```
https://cluster-api.sigs.k8s.io/developer/providers/migrations/v1.OLD_CAPI_MINOR-to-v1.NEW_CAPI_MINOR
```

For example, for a jump from CAPI v1.12 to v1.13: `https://cluster-api.sigs.k8s.io/developer/providers/migrations/v1.12-to-v1.13`

Do NOT apply everything listed there blindly. Instead:

- **Removals** and **API changes**: these are mandatory. Fix anything that applies to this project — the build will likely fail otherwise.
- **Deprecation**, **Cluster API Contract changes**, and **Suggested changes for providers**: review these carefully. Evaluate whether each suggestion applies to this project and whether it makes sense to adopt now. Some may be best deferred to a follow-up.

## Step 3: Update go.mod (root)

Update the direct dependencies:

```
sigs.k8s.io/cluster-api CAPI_VERSION
sigs.k8s.io/cluster-api/test CAPI_VERSION
sigs.k8s.io/controller-runtime CONTROLLER_RUNTIME_VER
k8s.io/api K8S_MODULE_VER
k8s.io/apimachinery K8S_MODULE_VER
k8s.io/client-go K8S_MODULE_VER
k8s.io/component-base K8S_MODULE_VER
github.com/GoogleCloudPlatform/k8s-cloud-provider K8S_CLOUD_PROVIDER_VER
```

Then run:

```bash
go mod tidy
```

Indirect dependencies, including Kind, will be resolved automatically.

## Step 4: Update hack/tools/go.mod

Update the `sigs.k8s.io/cluster-api/hack/tools` pseudo-version. To find the right version:

```bash
GOPROXY=https://proxy.golang.org go list -m -json "sigs.k8s.io/cluster-api/hack/tools@CAPI_VERSION"
```

Update the setup-envtest tool pin to the version found in Step 1c, then run:

```bash
cd hack/tools
go get -tool sigs.k8s.io/controller-runtime/tools/setup-envtest@SETUP_ENVTEST_VER
go mod tidy
```

## Step 5: Verify derived tool versions

Do not manually edit `KUBEBUILDER_ENVTEST_KUBERNETES_VERSION`, `CONTROLLER_GEN_VER`, `CONVERSION_GEN_VER`, `KIND_VER`, `KUBECTL_VER`, or `SETUP_ENVTEST_VER` in the Makefile. They follow the root or tools module:

```bash
go list -m k8s.io/client-go sigs.k8s.io/kind github.com/onsi/ginkgo/v2
(cd hack/tools && go list -m sigs.k8s.io/controller-tools k8s.io/code-generator sigs.k8s.io/controller-runtime/tools/setup-envtest)
```

The `k8s.io/client-go` version `v0.MINOR.PATCH` determines kubectl `v1.MINOR.PATCH` and the envtest `1.MINOR` selector. Verify the kubectl binary and a matching envtest release are available. The Ginkgo CLI follows the root `go.mod`; controller-gen, conversion-gen, and setup-envtest follow `hack/tools/go.mod`. Check that their selected versions are appropriate before regenerating files.

Do NOT update the independent pins `GOLANG_VERSION`, `KUSTOMIZE_VER`, `CERT_MANAGER_VER`, or `CALICO_VERSION` as part of the k8s/CAPI bump. `GOLANGCI_LINT_VER` follows `.github/workflows/lint.yml` and is handled by the Go bump guide.

## Step 6: Update metadata.yaml

Add a new release series entry at the end of the list for the new CAPG minor version:

```yaml
  - major: 1
    minor: NEW_CAPI_MINOR
    contract: v1beta1
```

## Step 7: Update test/e2e/data/shared/v1beta1/metadata.yaml

Add a new release series entry at the **top** of the list (this file is ordered newest-first):

```yaml
  - major: 1
    minor: NEW_CAPI_MINOR
    contract: v1beta1
```

## Step 8: Update test/e2e/config/gcp-ci.yaml

### Provider versions

Update CAPI core, bootstrap, and control-plane provider versions and URLs from `OLD_CAPI_VERSION` to `CAPI_VERSION`.

Update the GCP infrastructure provider dev version from `v1.OLD_CAPI_MINOR.99` to `v1.NEW_CAPI_MINOR.99`.

### Variables

```yaml
KUBERNETES_MINOR: "K8S_MINOR"
```

That's the only line to touch here. Every other Kubernetes-version
variable in this file (`KUBERNETES_VERSION`, `KUBERNETES_VERSION_GKE`,
`CCM_VERSION`, `KUBERNETES_VERSION_MANAGEMENT`, the upgrade-test FROM/TO/
etcd/coredns/image variables) is already `"${VAR}"` with no default, and
derives automatically at CI time via `hack/resolve-e2e-versions.sh` — don't
add a hand-pinned fallback to any of them; that would just recreate the
staleness problem this whole scheme exists to avoid. Preserve the comments
above `KUBERNETES_MINOR` in the YAML — they explain why the rest of the
block has no defaults.

Optionally, run `hack/resolve-e2e-versions.sh` locally first (needs
`gcloud`/`docker`/`git` access; `E2E_FLAVOR=all GCP_PROJECT=... GCP_REGION=...`)
to catch a missing nightly image, CCM tag, or kindest/node image before
pushing, rather than waiting on a full CI run to find out.

## Step 9: Update CCM manifest

`test/e2e/data/ccm/gce-cloud-controller-manager.yaml` already references
`${CCM_VERSION}` with no default — nothing to change here either, for the
same reason as Step 8.

## Step 10: Regenerate CRDs

Changes to the derived controller-gen or conversion-gen versions may update generated files:

```bash
make generate
make manifests
```

## Step 11: Build and verify

```bash
go build ./...
```

If `go build` fails with API changes (e.g. breaking changes in controller-runtime or CAPI), fix the Go source files to match the new API using the migration guide from Step 2.

## Step 12: Fix lint issues

```bash
make lint
```

If lint reports deprecation warnings (e.g. `SA1019` for deprecated interfaces or types), fix what can be fixed (migrate to new APIs) and add `.golangci.yml` exclusions for deprecations that cannot be resolved yet (e.g. upstream CAPI types still using the deprecated form).

## Step 13: Run tests

```bash
make test
```

## Step 14: Branch and commit

Create a branch named `bump-k8s-MINOR-capi-MINOR` (e.g. `bump-k8s-135-capi-113`) and commit the changes as two separate commits:

1. The version bump itself:
   ```
   chore(bump): bump k8s to K8S_MINOR, CAPI to CAPI_VERSION
   ```

2. Lint and build fixes (if any):
   ```
   fix(lint): <describe the migration or fix>
   ```

Do NOT push or create a PR unless the user asks. When creating a PR, follow the template in `.github/PULL_REQUEST_TEMPLATE.md`.
