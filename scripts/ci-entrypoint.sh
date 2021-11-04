#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
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

###############################################################################

# To run locally, set GCP_CLIENT_ID, GCP_CLIENT_SECRET, GCP_SUBSCRIPTION_ID, GCP_TENANT_ID

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"

# building kubectl from tools folder
mkdir -p "${REPO_ROOT}/hack/tools/bin"
KUBECTL=$(realpath hack/tools/bin/kubectl)
# export the variable so it is available in bash -c wait_for_nodes below
export KUBECTL
make "${KUBECTL}" &>/dev/null
echo "KUBECTL is set to ${KUBECTL}"

# shellcheck source=hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"
# shellcheck source=hack/parse-prow-creds.sh
source "${REPO_ROOT}/hack/parse-prow-creds.sh"
# shellcheck source=hack/util.sh
source "${REPO_ROOT}/hack/util.sh"

get_random_region=(){
    local REGIONS=("asia-east1-a" "asia-east2-b" "asia-northeast3-c" "asia-south1-a" "europe-north1-a" "europe-west3-c" "northamerica-northeast2-b")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

setup(){
	# setup REGISTRY for custom images.
    : "${REGISTRY:?Environment variable empty or not defined.}"
    if [[ -z "${CLUSTER_TEMPLATE:-}" ]]; then
        select_cluster_template
    fi

    export GCP_REGION="<GCP_REGION>"
    export GCP_PROJECT="<GCP_PROJECT>"
    export KUBERNETES_VERSION=1.20.9
    export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
    export GCP_NODE_MACHINE_TYPE=n1-standard-2
    export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default>
    export CLUSTER_NAME="<CLUSTER_NAME>"
}

wait_for_nodes() {
    echo "Waiting for ${GCP_CONTROL_PLANE_MACHINE_TYPE} gcp control plane machine(s) and ${GCP_NODE_MACHINE_TYPE} gcp node machine type to become ready."
    # Ensure that all nodes are registered with the API server before checking for readiness
    local total_nodes="$((GCP_CONTROL_PLANE_MACHINE_TYPE + GCP _NODE_MACHINE_TYPE))"
     while [[ $("${KUBECTL}" get nodes -ojson | jq '.items | length') -ne "${total_nodes}" ]]; do
        sleep 10
    done

    "${KUBECTL}" wait --for=condition=Ready node --all --timeout=5m
    "${KUBECTL}" get nodes -owide
}

# cleanup all resources we use
cleanup() {
    timeout 1800 "${KUBECTL}" delete cluster "${CLUSTER_NAME}" || true
    make kind-reset || true
}

on_exit() {
    unset KUBECONFIG
    # cleanup
    if [[ -z "${SKIP_CLEANUP:-}" ]]; then
        cleanup
    fi
}

# setup all required variables and images
setup

trap on_exit EXIT
export ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"

# create cluster
create_cluster

# export the target cluster KUBECONFIG if not already set
export KUBECONFIG="${KUBECONFIG:-${PWD}/kubeconfig}"

export -f wait_for_nodes
timeout --foreground 1800 bash -c wait_for_nodes

if [[ "${#}" -gt 0 ]]; then
    # disable error exit so we can run post-command cleanup
    set +o errexit
    "${@}"
    EXIT_VALUE="${?}"
    exit ${EXIT_VALUE}
fi


