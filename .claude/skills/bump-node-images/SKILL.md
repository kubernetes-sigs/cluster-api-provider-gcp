---
description: |
  Bump the node (VM) images used by the e2e and conformance jobs: the Ubuntu
  version, or the pinned Flatcar LTS image.
  Use when moving to a new Ubuntu LTS (e.g. from 2204 to 2404), when
  image-builder drops the current Ubuntu target, or when the pinned Flatcar
  image has been purged or a newer Flatcar LTS is wanted.
  Handles the CAPG_UBUNTU_VERSION and CAPG_FLATCAR_VERSION pins in gcp-ci.yaml and the docs that name the image.
argument-hint: "<ubuntu|flatcar> <version> (e.g. ubuntu 2404, flatcar 4081.3.10)"
allowed-tools: Bash(git *) Bash(grep *) Bash(gh *) Bash(gcloud *) Bash(make *) Bash(shellcheck *) Bash(curl *) Bash(hack/tools/bin/yq *)
---

# Bump Node Images

Target: **$ARGUMENTS** (the image family, `ubuntu` or `flatcar`, followed by the new version)

Follow the instructions in [docs/book/src/developers/bump-node-images.md](../../../docs/book/src/developers/bump-node-images.md) exactly, using the section for the given image family and `$ARGUMENTS` as the target version.