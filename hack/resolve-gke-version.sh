#!/usr/bin/env bash

# Copyright The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script resolves KUBERNETES_VERSION_GKE to the latest version GKE's
# regular channel currently offers for the KUBERNETES_MINOR_GKE pinned in
# test/e2e/config/gcp-ci.yaml, so the patch doesn't go stale as GKE moves
# the channel forward. It is meant to be sourced, not executed directly.
# Required input: GCP_PROJECT and GCP_REGION must be set in the caller's env.
# Skipped if GINKGO_FOCUS is explicitly set and excludes GKE specs.

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [ -z "${GCP_PROJECT:-}" ]; then echo "ERROR: GCP_PROJECT is not set" >&2; return 1; fi
if [ -z "${GCP_REGION:-}" ]; then echo "ERROR: GCP_REGION is not set" >&2; return 1; fi

case "${GINKGO_FOCUS:-}" in
  *GKE*|"") ;;
  *)
    echo "GINKGO_FOCUS (${GINKGO_FOCUS}) excludes GKE specs; skipping GKE version resolution."
    return 0
    ;;
esac

# Read the target minor from the raw config file, not an env var -- this
# script runs before the Makefile's envsubst step, so there's nothing in
# the environment to read yet. Same approach ci-e2e.sh already uses for
# KUBERNETES_VERSION.
minor=$("${REPO_ROOT}/hack/tools/bin/yq" '.variables.KUBERNETES_MINOR_GKE' "${REPO_ROOT}/test/e2e/config/gcp-ci.yaml")
if [ -z "${minor}" ] || [ "${minor}" = "null" ]; then
  echo "ERROR: KUBERNETES_MINOR_GKE is not set in test/e2e/config/gcp-ci.yaml" >&2
  return 1
fi
escaped_minor=${minor//./\\.}

resolved=$(gcloud container get-server-config \
  --project="${GCP_PROJECT}" --region="${GCP_REGION}" --format=json \
  | jq -r '.channels[] | select(.channel=="REGULAR") | .validVersions[]' \
  | grep -E "^${escaped_minor}\." \
  | sort -V | tail -1)
if [ -z "${resolved}" ]; then
  echo "ERROR: GKE's regular channel has no valid version for minor ${minor} yet" >&2
  return 1
fi

KUBERNETES_VERSION_GKE="v${resolved%%-*}"
export KUBERNETES_VERSION_GKE

echo "Resolved KUBERNETES_VERSION_GKE=${KUBERNETES_VERSION_GKE} from GKE's regular channel (minor ${minor})."
