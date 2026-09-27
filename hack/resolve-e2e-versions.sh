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

# This script derives every Kubernetes-version-related variable in
# test/e2e/config/gcp-ci.yaml from the single KUBERNETES_MINOR pinned
# there, instead of each one being a separately hand-maintained literal
# that can independently go stale. It is meant to be sourced, not executed
# directly. Required input: GCP_PROJECT and GCP_REGION must be set in the
# caller's env. Which variables get resolved depends on E2E_FLAVOR
# (all|gke|unmanaged|upgrade, set via scripts/ci-e2e.sh's --flavor arg;
# defaults to "all").

if [ -z "${GCP_PROJECT:-}" ]; then echo "ERROR: GCP_PROJECT is not set" >&2; return 1; fi
if [ -z "${GCP_REGION:-}" ]; then echo "ERROR: GCP_REGION is not set" >&2; return 1; fi

minor=$(go run github.com/mikefarah/yq/v4@v4.45.4 '.variables.KUBERNETES_MINOR' test/e2e/config/gcp-ci.yaml)
if [ -z "${minor}" ] || [ "${minor}" = "null" ]; then
  echo "ERROR: KUBERNETES_MINOR is not set in test/e2e/config/gcp-ci.yaml" >&2
  return 1
fi

# k8s_tags_for_minor prints kubernetes/kubernetes release tags for a given
# minor (e.g. "1.35"), newest first.
k8s_tags_for_minor() {
  local m=$1 escaped
  escaped=${m//./\\.}
  git ls-remote --tags --refs https://github.com/kubernetes/kubernetes \
    | awk -F/ '{print $3}' | grep -E "^v${escaped}\.[0-9]+$" | sort -rV
}

# find_verified_version prints the first candidate (from stdin, newest
# first) for which the given verification command succeeds, or fails if
# none do.
find_verified_version() {
  local verify_cmd=$1 candidate
  while read -r candidate; do
    if "${verify_cmd}" "${candidate}"; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}

verify_nightly_image() {
  gcloud compute images describe "cluster-api-ubuntu-2204-${1//./-}-nightly" \
    --project=k8s-staging-cluster-api-gcp >/dev/null 2>&1
}

verify_ccm_image() {
  docker manifest inspect "gcr.io/k8s-staging-cloud-provider-gcp/cloud-controller-manager:${1}" >/dev/null 2>&1
}

verify_kindest_node() {
  docker manifest inspect "kindest/node:${1}" >/dev/null 2>&1
}

# resolve_version_for_minor finds the latest release of the given minor
# that's actually usable (per verify_cmd), and dies with a specific,
# actionable message if none is -- these are all cases where the missing
# piece lives in some other repo (image-builder, cloud-provider-gcp,
# kindest/node) that nothing here can create, so the fix is "go add it
# there", not a bug in this script.
resolve_version_for_minor() {
  local minor=$1 verify_cmd=$2 not_found_msg=$3 resolved
  if ! resolved=$(k8s_tags_for_minor "${minor}" | find_verified_version "${verify_cmd}"); then
    echo "ERROR: ${not_found_msg}" >&2
    return 1
  fi
  printf '%s\n' "${resolved}"
}

resolve_unmanaged_version() {
  local resolved
  resolved=$(resolve_version_for_minor "${minor}" verify_nightly_image \
    "no nightly image published for minor ${minor} -- add config at kubernetes-sigs/image-builder/images/capi/packer/gce/ci/nightly") || return 1
  KUBERNETES_VERSION="${resolved}"
  IMAGE_ID="projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-${resolved//./-}-nightly"
  export KUBERNETES_VERSION IMAGE_ID

  resolve_ccm_version || return 1
}

resolve_ccm_version() {
  local major=${minor#*.} resolved
  resolved=$(git ls-remote --tags --refs https://github.com/kubernetes/cloud-provider-gcp \
      | awk -F/ '{print $3}' | grep -E "^v${major}\." | sort -rV \
      | find_verified_version verify_ccm_image) \
    || { echo "ERROR: no published cloud-controller-manager image found for k8s minor ${minor} (CCM major v${major})" >&2; return 1; }
  CCM_VERSION="${resolved}"
  export CCM_VERSION
}

resolve_management_version() {
  local resolved
  if ! resolved=$(k8s_tags_for_minor "${minor}" | find_verified_version verify_kindest_node); then
    echo "ERROR: no kindest/node image found for minor ${minor}" >&2
    return 1
  fi
  KUBERNETES_VERSION_MANAGEMENT="${resolved}"
  export KUBERNETES_VERSION_MANAGEMENT
}

resolve_gke_version() {
  local resolved
  resolved=$(gcloud container get-server-config \
      --project="${GCP_PROJECT}" --region="${GCP_REGION}" --format=json \
    | jq -r '.channels[] | select(.channel=="REGULAR") | .validVersions[]' \
    | grep -E "^${minor//./\\.}\." | sort -V | tail -1)
  if [ -z "${resolved}" ]; then
    echo "ERROR: GKE's regular channel has no valid version for minor ${minor} -- don't bump KUBERNETES_MINOR past what GKE supports" >&2
    return 1
  fi
  KUBERNETES_VERSION_GKE="v${resolved%%-*}"
  export KUBERNETES_VERSION_GKE
}

resolve_upgrade_versions() {
  local prev_major prev_minor to_v from_v constants
  resolve_ccm_version || return 1
  to_v=$(resolve_version_for_minor "${minor}" verify_nightly_image \
    "no nightly image published for minor ${minor} -- add config at kubernetes-sigs/image-builder/images/capi/packer/gce/ci/nightly") || return 1

  prev_major=${minor%.*}
  prev_minor=$(( ${minor#*.} - 1 ))
  from_v=$(resolve_version_for_minor "${prev_major}.${prev_minor}" verify_nightly_image \
    "no nightly image published for the previous minor ${prev_major}.${prev_minor} -- add config at kubernetes-sigs/image-builder/images/capi/packer/gce/ci/nightly") || return 1

  KUBERNETES_VERSION_UPGRADE_TO="${to_v}"
  KUBERNETES_VERSION_UPGRADE_FROM="${from_v}"
  KUBERNETES_IMAGE_UPGRADE_TO="projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-${to_v//./-}-nightly"
  KUBERNETES_IMAGE_UPGRADE_FROM="projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-${from_v//./-}-nightly"
  export KUBERNETES_VERSION_UPGRADE_TO KUBERNETES_VERSION_UPGRADE_FROM
  export KUBERNETES_IMAGE_UPGRADE_TO KUBERNETES_IMAGE_UPGRADE_FROM

  constants=$(curl -sf "https://raw.githubusercontent.com/kubernetes/kubernetes/${to_v}/cmd/kubeadm/app/constants/constants.go")
  if [ -z "${constants}" ]; then
    echo "ERROR: couldn't fetch kubeadm constants.go at ${to_v} to resolve etcd/coredns versions" >&2
    return 1
  fi
  ETCD_VERSION_UPGRADE_TO=$(printf '%s\n' "${constants}" | grep 'DefaultEtcdVersion = ' | sed -E 's/.*"([^"]+)".*/\1/')
  COREDNS_VERSION_UPGRADE_TO=$(printf '%s\n' "${constants}" | grep 'CoreDNSVersion = ' | sed -E 's/.*"([^"]+)".*/\1/')
  export ETCD_VERSION_UPGRADE_TO COREDNS_VERSION_UPGRADE_TO
}

# print_summary prints every version this script resolved, in one place,
# right before the caller moves on -- so anyone reading CI output (or a
# local run) can find what actually ran with a single glance/search,
# rather than piecing it together from scattered lines earlier in a very
# long log.
print_summary() {
  echo "---- resolved e2e versions (flavor=${E2E_FLAVOR:-all}, KUBERNETES_MINOR=${minor}) ----"
  local v
  for v in KUBERNETES_VERSION IMAGE_ID KUBERNETES_VERSION_GKE CCM_VERSION \
      KUBERNETES_VERSION_MANAGEMENT KUBERNETES_VERSION_UPGRADE_FROM \
      KUBERNETES_VERSION_UPGRADE_TO KUBERNETES_IMAGE_UPGRADE_FROM \
      KUBERNETES_IMAGE_UPGRADE_TO ETCD_VERSION_UPGRADE_TO COREDNS_VERSION_UPGRADE_TO; do
    if [ -n "${!v:-}" ]; then
      printf '  %s=%s\n' "${v}" "${!v}"
    fi
  done
  echo "-------------------------------------------------------------"
}

case "${E2E_FLAVOR:-all}" in
  all)
    resolve_unmanaged_version || return 1
    case "${GINKGO_FOCUS:-}" in
      *GKE*|"") resolve_gke_version || return 1 ;;
      *)
        echo "GINKGO_FOCUS (${GINKGO_FOCUS}) excludes GKE specs; skipping GKE version resolution."
        ;;
    esac
    resolve_upgrade_versions || return 1
    ;;
  gke)       resolve_gke_version ;;
  unmanaged) resolve_unmanaged_version ;;
  upgrade)   resolve_upgrade_versions ;;
  *)         echo "ERROR: unknown E2E_FLAVOR '${E2E_FLAVOR}'" >&2; return 1 ;;
esac || return 1

resolve_management_version || return 1

print_summary
