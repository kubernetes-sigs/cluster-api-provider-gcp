# Bumping Go

This document describes how to bump the Go version across the project. It is primarily intended to be consumed by an AI coding agent (e.g. via `/bump-go 1.26`), but the steps can also be followed manually.

## Convention

- `go` directive in go.mod: use `X.Y.0` (the minor version with patch 0)
- `toolchain` directive in go.mod: use `goX.Y.Z` where Z is the **latest available patch**

## Step 1: Research

Perform these lookups before making any changes.

### 1a. Latest patch version

Find the latest Go `X.Y.x` patch release at <https://go.dev/doc/devel/release>.
Determine the full version string (e.g. `1.26.4`). Call this `FULL_VERSION`.

### 1b. Docker image digest

Pull the official golang image and extract its digest:

```bash
docker pull golang:FULL_VERSION
docker inspect --format='{{index .RepoDigests 0}}' golang:FULL_VERSION
```

Extract the `sha256:...` digest. Call this `GOLANG_DIGEST`.

### 1c. Upstream cluster-api references

Look up what `kubernetes-sigs/cluster-api` uses at the latest release tag on the main CAPI minor version this project depends on (check `go.mod` for `sigs.k8s.io/cluster-api` to find the CAPI minor, then find the latest tag for that minor, e.g. v1.13.2):

```bash
# GCB image digest + tag comment
gh api "repos/kubernetes-sigs/cluster-api/contents/cloudbuild.yaml?ref=<TAG>" --jq '.content' | base64 -d

# golangci-lint version
gh api "repos/kubernetes-sigs/cluster-api/contents/.github/workflows/pr-golangci-lint.yaml?ref=<TAG>" --jq '.content' | base64 -d | grep 'version:'
```

Call these `GCB_DIGEST`, `GCB_TAG_COMMENT`, and `CAPI_GOLANGCI_VER`.

### 1d. golangci-lint compatibility

The golangci-lint version must be **built with the target Go version** or newer. Versions built with an older Go will refuse to lint. Test candidate versions:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<VERSION> version
```

Look for `built with goX.Y` in the output. Pick the latest stable version that reports the target Go minor or newer. Call this `GOLANGCI_VERSION` (e.g. `v2.12.2`) and `GOLANGCI_MINOR` (e.g. `v2.12`).

### 1e. Delve version

Delve major version should match the Go minor version (e.g. Go 1.26 → dlv v1.26). Verify at <https://github.com/go-delve/delve/releases> that a matching tag exists. If not, use the latest available. Call this `DLV_VERSION`.

## Step 2: Update files

### go.mod (root) and hack/tools/go.mod

```
go X.Y.0
toolchain goFULL_VERSION
```

### Makefile

```makefile
GOLANG_VERSION := FULL_VERSION
GOLANG_DIRECTIVE_VERSION ?= X.Y.0
GOLANGCI_LINT_VER := GOLANGCI_VERSION
```

### Dockerfile

Update the FROM line, replacing both the tag and the digest:

```dockerfile
FROM golang:FULL_VERSION@sha256:GOLANG_DIGEST as builder
```

### Tiltfile

Two `FROM golang:` lines (tilt-helper and tilt) and the delve install:

```
FROM golang:FULL_VERSION as tilt-helper
RUN go install github.com/go-delve/delve/cmd/dlv@DLV_VERSION
...
FROM golang:FULL_VERSION as tilt
```

### netlify.toml

```toml
GO_VERSION = "FULL_VERSION"
```

### .github/workflows/lint.yml

```yaml
go-version: "X.Y"
...
version: GOLANGCI_MINOR
```

### cloudbuild.yaml and cloudbuild-nightly.yaml

Update the GCB image digest and tag comment:

```yaml
- name: 'gcr.io/k8s-staging-test-infra/gcb-docker-gcloud@sha256:GCB_DIGEST' # GCB_TAG_COMMENT
```

## Step 3: go mod tidy

Run in both module directories:

```bash
go mod tidy
cd hack/tools && go mod tidy
```

## Step 4: Lint and test

```bash
make lint
make test
```

If `make lint` fails:
- If the failure is `the Go language version used to build golangci-lint is lower than the targeted Go version`, the chosen `GOLANGCI_VERSION` is wrong. Go back to Step 1d.
- If the failure shows new lint findings, fix them. These are pre-existing issues surfaced by the newer linter version, not caused by the Go bump itself. Common categories:
  - **goconst in test files**: add an exclusion in `.golangci.yml` (upstream cluster-api excludes goconst from `_test.go`)
  - **goconst in production code**: extract repeated string literals into package-level constants
  - **prealloc**: change `var s []T` or `s := []T{}` to `s := make([]T, 0, len(source))`
  - **perfsprint**: replace string concatenation in loops with `strings.Builder`
  - **staticcheck**: follow the suggested fix (e.g. `fmt.Fprintf` instead of `WriteString(fmt.Sprintf)`)

## Step 5: Verify build

```bash
go build ./...
```

## Step 6: Commit

Create **three separate commits** in this order:

1. **Lint fixes** (if any): `fix(lint): resolve <linter-names> lint issues`
   - Only the source files with lint fixes
2. **Linter bump** (if version changed): `chore(bump): bump golangci-lint from <old> to <new>`
   - `.golangci.yml`, `Makefile` (GOLANGCI_LINT_VER only), `.github/workflows/lint.yml` (version only)
3. **Go version bump**: `chore(bump): bump Go to FULL_VERSION`
   - All remaining files: go.mod, hack/tools/go.mod, Makefile (GOLANG_VERSION + GOLANG_DIRECTIVE_VERSION), Dockerfile, Tiltfile, netlify.toml, cloudbuild*.yaml, .github/workflows/lint.yml (go-version only)

If there are no lint fixes or no linter version change, skip that commit.

Do NOT push or create a PR unless the user asks.
