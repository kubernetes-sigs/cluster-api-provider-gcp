#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
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

################################################################################
# usage: setup-dev-environment.sh
#  This program inits network in GCP and starts tilt
################################################################################
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"

# Verify the required Environment Variables are present.
: "${GCP_REGION:?Environment variable empty or not defined.}"
: "${GCP_PROJECT:?Environment variable empty or not defined.}"
: "${CONTROL_PLANE_MACHINE_COUNT:?Environment variable empty or not defined.}"
: "${WORKER_MACHINE_COUNT:?Environment variable empty or not defined.}"
: "${KUBERNETES_VERSION:?Environment variable empty or not defined.}"
: "${GCP_CONTROL_PLANE_MACHINE_TYPE:?Environment variable empty or not defined.}"
: "${GCP_NODE_MACHINE_TYPE:?Environment variable empty or not defined.}"
: "${GCP_NETWORK_NAME:?Environment variable empty or not defined.}"
: "${CLUSTER_NAME:?Environment variable empty or not defined.}"
: "${GCP_B64ENCODED_CREDENTIALS:?Environment variable empty or not defined.}"

# initialize a router and cloud NAT
init_networks() {
  if [[ ${GCP_NETWORK_NAME} != "default" ]]; then
    gcloud compute networks create --project "$GCP_PROJECT" "${GCP_NETWORK_NAME}" --subnet-mode auto --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-http --project "$GCP_PROJECT" \
      --allow tcp:80 --network "${GCP_NETWORK_NAME}" --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-https --project "$GCP_PROJECT" \
      --allow tcp:443 --network "${GCP_NETWORK_NAME}" --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-icmp --project "$GCP_PROJECT" \
      --allow icmp --network "${GCP_NETWORK_NAME}" --priority 65534 --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-internal --project "$GCP_PROJECT" \
      --allow "tcp:0-65535,udp:0-65535,icmp" --network "${GCP_NETWORK_NAME}" --priority 65534 --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-rdp --project "$GCP_PROJECT" \
      --allow "tcp:3389" --network "${GCP_NETWORK_NAME}" --priority 65534 --quiet
    gcloud compute firewall-rules create "${GCP_NETWORK_NAME}"-allow-ssh --project "$GCP_PROJECT" \
      --allow "tcp:22" --network "${GCP_NETWORK_NAME}" --priority 65534 --quiet
  fi

  gcloud compute firewall-rules list --project "$GCP_PROJECT"
  gcloud compute networks list --project="${GCP_PROJECT}"
  gcloud compute networks describe "${GCP_NETWORK_NAME}" --project="${GCP_PROJECT}"

  gcloud compute routers create "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --network="${GCP_NETWORK_NAME}"
  gcloud compute routers nats create "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter" \
    --nat-all-subnet-ip-ranges --auto-allocate-nat-external-ips
}

cleanup_network() {
  # cleanup the networks
  gcloud compute routers nats delete "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter" --quiet || true
  gcloud compute routers delete "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --quiet || true

  if [[ ${GCP_NETWORK_NAME} != "default" ]]; then
    (gcloud compute firewall-rules list --project "$GCP_PROJECT" | grep "$GCP_NETWORK_NAME" \
         | awk '{print "gcloud compute firewall-rules delete --project '"$GCP_PROJECT"' --quiet " $1 "\n"}' \
         | bash) || true
    gcloud compute networks delete --project="${GCP_PROJECT}" \
      --quiet "${GCP_NETWORK_NAME}" || true
  fi
}


# setup network and start tilt
main() {
  for arg in "$@"
  do
    if [[ "$arg" == "--skip-init-network" ]]; then
      SKIP_INIT_NETWORK="1"
    fi

    if [[ "$arg" == "--clean-network" ]]; then
      cleanup_network
      return
    fi
  done

  # Initialize the necessary network requirements
  if [[ -n "${SKIP_INIT_NETWORK:-}" ]]; then
    echo "Skipping network initialization..."
  else
    init_networks
  fi

  make tilt-up
}

main "$@"
