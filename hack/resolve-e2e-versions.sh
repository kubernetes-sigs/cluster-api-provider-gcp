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
# directly. Various helper functions are located in hack/lib/versions.sh
#
# Required input: GCP_PROJECT and GCP_REGION must be set in the
# caller's env. Which variables get resolved depends on E2E_FLAVOR
# (all|gke|unmanaged|upgrade, set via scripts/ci-e2e.sh's --flavor arg;
# defaults to "all").

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [ -z "${GCP_PROJECT:-}" ]; then echo "ERROR: GCP_PROJECT is not set" >&2; return 1; fi
if [ -z "${GCP_REGION:-}" ]; then echo "ERROR: GCP_REGION is not set" >&2; return 1; fi

# shellcheck source=hack/lib/versions.sh
source "${REPO_ROOT}/hack/lib/versions.sh"
load_ci_pins || return 1

verify_ccm_image() {
  docker manifest inspect "gcr.io/k8s-staging-cloud-provider-gcp/cloud-controller-manager:${1}" >/dev/null
}

verify_cluster_autoscaler_image() {
  docker manifest inspect "registry.k8s.io/autoscaling/cluster-autoscaler:${1}" >/dev/null
}

verify_kindest_node() {
  docker manifest inspect "kindest/node:${1}" >/dev/null
}

resolve_unmanaged_version() {
  resolve_node_image || return 1
  resolve_ccm_version || return 1
}

resolve_ccm_version() {
  local major=${minor#*.} resolved
  resolved=$(get_tag_list "https://github.com/kubernetes/cloud-provider-gcp" \
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

resolve_cluster_autoscaler_version() {
  local resolved escaped
  escaped=${minor//./\\.}
  resolved=$(get_tag_list https://github.com/kubernetes/autoscaler \
      | awk -F/ '{print $3}' | grep -E "^cluster-autoscaler-${escaped}\.[0-9]+$" | sort -rV \
      | sed 's/^cluster-autoscaler-/v/' \
      | find_verified_version verify_cluster_autoscaler_image) \
    || { echo "ERROR: no published cluster-autoscaler image found for k8s minor ${minor}" >&2; return 1; }
  CLUSTER_AUTOSCALER_VERSION="${resolved}"
  export CLUSTER_AUTOSCALER_VERSION
}

resolve_gke_version() {
  local server_config_json
  local extracted_versions
  local resolved

  server_config_json=$(gcloud container get-server-config --project="${GCP_PROJECT}" --region="${GCP_REGION}" --format=json) || return 1
  extracted_versions=$(echo "$server_config_json" | jq -r '.channels[] | select(.channel=="REGULAR") | .validVersions[]') || return 1
  resolved=$(echo "$extracted_versions" | grep -E "^${minor//./\\.}\." | sort -V | tail -1)
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
  KUBERNETES_IMAGE_UPGRADE_TO=$(build_nightly_image_id "$to_v")
  KUBERNETES_IMAGE_UPGRADE_FROM=$(build_nightly_image_id "$from_v")
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
      KUBERNETES_VERSION_MANAGEMENT CLUSTER_AUTOSCALER_VERSION \
      KUBERNETES_VERSION_UPGRADE_FROM KUBERNETES_VERSION_UPGRADE_TO \
      KUBERNETES_IMAGE_UPGRADE_FROM KUBERNETES_IMAGE_UPGRADE_TO \
      ETCD_VERSION_UPGRADE_TO COREDNS_VERSION_UPGRADE_TO; do
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
resolve_cluster_autoscaler_version || return 1

print_summary
