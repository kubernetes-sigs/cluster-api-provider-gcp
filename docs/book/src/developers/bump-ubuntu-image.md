# Bumping the Ubuntu Image

This document describes how to bump the Ubuntu version of the VM images used by the e2e and conformance jobs. It is primarily intended to be consumed by an AI coding agent (e.g. via `/bump-ubuntu-image 2404`), but the steps can also be followed manually.

## Convention

The Ubuntu version lives in one place: `CAPG_UBUNTU_VERSION` in `../../../../test/e2e/config/gcp-ci.yaml`. Its value is the suffix of the image-builder target, so `"2204"` means `make build-gce-ubuntu-2204`.

`../../../../hack/resolve-e2e-versions.sh`, `scripts/ci-e2e.sh` and `scripts/ci-conformance.sh` all read the pin, so none of them should be edited for a bump. Everything Kubernetes-related is bumped separately, see [Bumping Kubernetes and Cluster API](./bump-k8s-capi.md).

Let `UBUNTU_VERSION` be the target version (e.g. `2404`).

## Step 1: Research

Perform these lookups before making any changes.

### 1a. image-builder supports the target

```bash
gh api repos/kubernetes-sigs/image-builder/contents/images/capi/packer/gce --jq '.[].name' | grep ubuntu
gh api repos/kubernetes-sigs/image-builder/contents/images/capi/Makefile --jq .content | base64 -d | grep '^GCE_BUILD_NAMES'
```

`ubuntu-UBUNTU_VERSION.json` must exist and `gce-ubuntu-UBUNTU_VERSION` must be in `GCE_BUILD_NAMES`. If not, stop: there is nothing to bump to yet.

### 1b. Nightly images exist for the target

The e2e jobs consume prebuilt images from the `k8s-staging-cluster-api-gcp` project, built daily by the image-builder project's [nightly job](https://prow.k8s.io/?job=periodic-image-builder-gcp-all-nightly). The job builds one set of images per config file in [`images/capi/packer/gce/ci/nightly`](https://github.com/kubernetes-sigs/image-builder/tree/main/images/capi/packer/gce/ci/nightly) (one per Kubernetes minor), using `build-gce-all`, so every target in `GCE_BUILD_NAMES` should be published.

A given Kubernetes patch only gets a nightly image if someone has added config for it there. `../../../../hack/resolve-e2e-versions.sh` searches for the latest patch that has one; it can't conjure a new one into existence.

Check that the target's images exist for the current minor (`KUBERNETES_MINOR` in `gcp-ci.yaml`) and the minors used by the upgrade tests:

```bash
gcloud compute images list --project k8s-staging-cluster-api-gcp --no-standard-images \
  --filter="name~cluster-api-ubuntu-UBUNTU_VERSION-.*-nightly" --format="value(name)"
```

If any minor the tests need is missing, the fix is in image-builder, not here. Don't bump until it is published, or the resolver will fail with "no nightly image published for minor ...".

### 1c. Current version

```bash
hack/tools/bin/yq -e '.variables.CAPG_UBUNTU_VERSION' test/e2e/config/gcp-ci.yaml
```

## Step 2: Update files

### test/e2e/config/gcp-ci.yaml

```yaml
CAPG_UBUNTU_VERSION: "UBUNTU_VERSION"
```

That's the only line to touch for the e2e and conformance jobs.

### Docs

These name the image directly and are not derived from the pin:

- `../prerequisites.md`: the `make build-gce-ubuntu-*` target and the `family:capi-ubuntu-*-k8s` filter
- `cluster-creation.md`: the example `IMAGE_ID`

Leave `../topics/autoscaling.md` alone: its `imageFamily` is a Google public image family in an unrelated example.

### Check nothing is left behind

```bash
rg -n 'ubuntu-?[0-9]{4}' scripts hack docs test --glob '!*.sum'
```

Every hit for the old version should be gone, apart from the unrelated autoscaling example above.

## Step 3: Verify

```bash
shellcheck scripts/ci-e2e.sh scripts/ci-conformance.sh hack/resolve-e2e-versions.sh
```

Only pre-existing info-level findings (SC1091, SC2329) are expected.

Then run the resolver to confirm the nightly images resolve (needs `gcloud`/`docker`/`git` access):

```bash
E2E_FLAVOR=all GCP_PROJECT=<project> GCP_REGION=<region> source hack/resolve-e2e-versions.sh
```

The summary it prints should show `IMAGE_ID` and the upgrade images using the new version.

Optionally, build an image locally with `scripts/ci-e2e.sh --init-image --build-image-only`. This needs GCP credentials and an image-builder checkout in `$GOPATH/src/sigs.k8s.io/image-builder`, and creates billable resources, so only do it if asked.

## Step 4: Commit

Create a single commit: `chore(bump): bump e2e Ubuntu image to UBUNTU_VERSION`, containing `gcp-ci.yaml` and the docs changes.

Do NOT push or create a PR unless the user asks.