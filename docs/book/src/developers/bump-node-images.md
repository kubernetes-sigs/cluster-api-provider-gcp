# Bumping Node Images

This document describes how to bump the VM images used by the e2e and conformance jobs: the Ubuntu version, or the pinned Flatcar LTS image. It is primarily intended to be consumed by an AI coding agent (e.g. via `/bump-node-images ubuntu 2404` or `/bump-node-images flatcar 4081.3.10`), but the steps can also be followed manually.

## Convention

Each image family is pinned in one place, in the "Hand-Edited Version Inputs" group of `../../../../test/e2e/config/gcp-ci.yaml`:

- `CAPG_UBUNTU_VERSION` is the suffix of the image-builder target, so `"2204"` means `make build-gce-ubuntu-2204`.
- `CAPG_FLATCAR_VERSION` is the version of a Flatcar LTS image published in the `kinvolk-public` project, so `"4081.3.10"` means the image `flatcar-lts-4081-3-10`.

`../../../../hack/resolve-e2e-versions.sh` reads both pins. `scripts/ci-e2e.sh` and `scripts/ci-conformance.sh` read the Ubuntu pin. None of them should be edited for a bump. Everything Kubernetes-related is bumped separately, see [Bumping Kubernetes and Cluster API](./bump-k8s-capi.md).

Use the section below that matches the image family you are bumping.

## Ubuntu

Let `UBUNTU_VERSION` be the target version (e.g. `2404`).

### Step 1: Research

Perform these lookups before making any changes.

#### 1a. image-builder supports the target

```bash
gh api repos/kubernetes-sigs/image-builder/contents/images/capi/packer/gce --jq '.[].name' | grep ubuntu
gh api repos/kubernetes-sigs/image-builder/contents/images/capi/Makefile --jq .content | base64 -d | grep '^GCE_BUILD_NAMES'
```

`ubuntu-UBUNTU_VERSION.json` must exist and `gce-ubuntu-UBUNTU_VERSION` must be in `GCE_BUILD_NAMES`. If not, stop: there is nothing to bump to yet.

#### 1b. Nightly images exist for the target

The e2e jobs consume prebuilt images from the `k8s-staging-cluster-api-gcp` project, built daily by the image-builder project's [nightly job](https://prow.k8s.io/?job=periodic-image-builder-gcp-all-nightly). The job builds one set of images per config file in [`images/capi/packer/gce/ci/nightly`](https://github.com/kubernetes-sigs/image-builder/tree/main/images/capi/packer/gce/ci/nightly) (one per Kubernetes minor), using `build-gce-all`, so every target in `GCE_BUILD_NAMES` should be published.

A given Kubernetes patch only gets a nightly image if someone has added config for it there. `../../../../hack/resolve-e2e-versions.sh` searches for the latest patch that has one; it can't conjure a new one into existence.

Check that the target's images exist for the current minor (`KUBERNETES_MINOR` in `gcp-ci.yaml`) and the minors used by the upgrade tests:

```bash
gcloud compute images list --project k8s-staging-cluster-api-gcp --no-standard-images \
  --filter="name~cluster-api-ubuntu-UBUNTU_VERSION-.*-nightly" --format="value(name)"
```

If any minor the tests need is missing, the fix is in image-builder, not here. Don't bump until it is published, or the resolver will fail with "no nightly image published for minor ...".

#### 1c. Current version

```bash
hack/tools/bin/yq -e '.variables.CAPG_UBUNTU_VERSION' test/e2e/config/gcp-ci.yaml
```

### Step 2: Update files

#### test/e2e/config/gcp-ci.yaml

```yaml
CAPG_UBUNTU_VERSION: "UBUNTU_VERSION"
```

That's the only line to touch for the e2e and conformance jobs.

#### Docs

These name the image directly and are not derived from the pin:

- `../prerequisites.md`: the `make build-gce-ubuntu-*` target and the `family:capi-ubuntu-*-k8s` filter
- `cluster-creation.md`: the example `IMAGE_ID`

Leave `../topics/autoscaling.md` alone: its `imageFamily` is a Google public image family in an unrelated example.

#### Check nothing is left behind

```bash
grep -rnEI 'ubuntu-?[0-9]{4}' scripts hack docs test --exclude='*.sum'
```

Every hit for the old version should be gone, apart from the unrelated autoscaling example above.

### Step 3: Verify

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

### Step 4: Commit

Create a single commit: `chore(bump): bump e2e Ubuntu image to UBUNTU_VERSION`, containing `gcp-ci.yaml` and the docs changes.

## Flatcar

Let `FLATCAR_VERSION` be the target LTS version (e.g. `4081.3.10`) and `FLATCAR_IMAGE` the image name derived from it by replacing the dots with dashes and prefixing `flatcar-lts-` (e.g. `flatcar-lts-4081-3-10`). Unlike the Ubuntu nightly images, Flatcar images have no `-nightly` suffix.

### Step 1: Research

#### 1a. The target version

The current LTS release is published on the Flatcar LTS release server:

```bash
curl -fsSL https://lts.release.flatcar-linux.net/amd64-usr/current/version.txt | grep '^FLATCAR_VERSION='
```

Use that value unless a specific version was requested.

#### 1b. The image exists

```bash
gcloud compute images describe FLATCAR_IMAGE --project kinvolk-public --format='value(name)'
```

If this fails, stop: either the image has not been published yet or Flatcar has purged it, and the resolver would fail with "Flatcar image ... could not be found". Listing the project is not permitted, but describing an image by name is.

#### 1c. Current version

```bash
hack/tools/bin/yq -e '.variables.CAPG_FLATCAR_VERSION' test/e2e/config/gcp-ci.yaml
```

### Step 2: Update files

#### test/e2e/config/gcp-ci.yaml

```yaml
CAPG_FLATCAR_VERSION: "FLATCAR_VERSION"
```

That's the only line to touch. `FLATCAR_IMAGE_ID` is derived by `../../../../hack/resolve-e2e-versions.sh` and the e2e spec reads it from the resolved config.

#### Docs

`../self-managed/flatcar.md` tells users to use the `family/flatcar-lts` image family, which always tracks the latest LTS, so it does not need changing for a bump within the LTS channel.

Moving to a different channel (for example `stable`) is a code change to `resolve_flatcar_version` in the resolver and to that page, not a bump.

#### Check nothing is left behind

```bash
grep -rnEI 'flatcar-(lts|stable)-[0-9]' hack test docs templates --exclude=bump-node-images.md
```

There should be no hits: the version lives only in the pin.

### Step 3: Verify

```bash
shellcheck hack/resolve-e2e-versions.sh
E2E_FLAVOR=unmanaged GCP_PROJECT=<project> GCP_REGION=<region> source hack/resolve-e2e-versions.sh
```

The summary it prints should show `FLATCAR_IMAGE_ID=projects/kinvolk-public/global/images/FLATCAR_IMAGE`.

Resolving the image does not prove the new Flatcar version boots a cluster. The Flatcar spec in the `e2e-test` presubmit does that, so check that it passes on the pull request.

### Step 4: Commit

Create a single commit: `chore(bump): bump e2e Flatcar image to FLATCAR_VERSION`, containing `gcp-ci.yaml`.

Do NOT push or create a PR unless the user asks.