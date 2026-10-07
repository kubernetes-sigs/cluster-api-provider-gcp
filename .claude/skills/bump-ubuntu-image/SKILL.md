---
description: |
  Bump the Ubuntu version of the VM images used by the e2e and conformance jobs.
  Use when moving to a new Ubuntu LTS (e.g. from 2204 to 2404) or when
  image-builder drops the current target.
  Handles the CAPG_UBUNTU_VERSION pin in gcp-ci.yaml and the docs that name the image.
argument-hint: "<ubuntu-version> (e.g. 2404)"
allowed-tools: Bash(git *) Bash(grep *) Bash(rg *) Bash(gh *) Bash(gcloud *) Bash(make *) Bash(shellcheck *) Bash(hack/tools/bin/yq *)
---

# Bump Ubuntu Image Version

Target Ubuntu version: **$ARGUMENTS**

Follow the instructions in [docs/book/src/developers/bump-ubuntu-image.md](../../../docs/book/src/developers/bump-ubuntu-image.md) exactly, using `$ARGUMENTS` as the target Ubuntu version.