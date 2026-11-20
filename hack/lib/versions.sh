#!/usr/bin/env bash

#
# Copyright 2026 The Kubernetes Authors.
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
#

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
HACK_DIR=$SCRIPT_DIR/..
TEST_DIR=$SCRIPT_DIR/../../test

load_ci_pins() {
  minor=$("${HACK_DIR}/tools/bin/yq" -e '.variables.KUBERNETES_MINOR' "${TEST_DIR}/e2e/config/gcp-ci.yaml") || return 1
  CAPG_UBUNTU_VERSION=$("${HACK_DIR}/tools/bin/yq" -e '.variables.CAPG_UBUNTU_VERSION' "${TEST_DIR}/e2e/config/gcp-ci.yaml") || return 1

  export CAPG_UBUNTU_VERSION
}

# get_tag_list grabs a list of tags from a Git repo and prints them
get_tag_list() {
  local repo=$1

  local tags
  tags=$(git ls-remote --tags --refs "$repo") || return 1
  if [ -z "$tags" ]; then
    echo "ERROR: Tag list from $repo was empty" >&2
    return 1
  fi
  echo "$tags"
}

# k8s_tags_for_minor prints kubernetes/kubernetes release tags for a given
# minor (e.g. "1.35"), newest first.
k8s_tags_for_minor() {
  local m=$1 escaped filtered
  escaped=${m//./\\.}
  filtered=$(get_tag_list "https://github.com/kubernetes/kubernetes" | awk -F/ '{print $3}' | grep -E "^v${escaped}\.[0-9]+$" | sort -rV)
  if [ -z "$filtered" ]; then
    echo "ERROR: After filtering to $m no versions remain" >&2
    return 1
  fi
  echo "$filtered"
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
  gcloud compute images describe "cluster-api-ubuntu-${CAPG_UBUNTU_VERSION}-${1//./-}-nightly" \
    --project=k8s-staging-cluster-api-gcp >/dev/null 2>&1
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

resolve_node_image() {
  local resolved
  resolved=$(resolve_version_for_minor "${minor}" verify_nightly_image \
    "no nightly image published for minor ${minor} -- add config at kubernetes-sigs/image-builder/images/capi/packer/gce/ci/nightly") || return 1
  KUBERNETES_VERSION="${resolved}"
  IMAGE_ID=$(build_nightly_image_id "$resolved")
  export KUBERNETES_VERSION IMAGE_ID
}

build_nightly_image_id() {
  local resolved=$1
  echo "projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-${CAPG_UBUNTU_VERSION}-${resolved//./-}-nightly"
}