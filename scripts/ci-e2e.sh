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

################################################################################
# usage: ci-e2e.sh
#  This program runs the e2e tests.
################################################################################

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECTL="${REPO_ROOT}/hack/tools/bin/kubectl"
KIND="${REPO_ROOT}/hack/tools/bin/kind"
make --directory="${REPO_ROOT}" "${KUBECTL##*/}" "${KIND##*/}"

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-kustomize.sh
source "${REPO_ROOT}/hack/ensure-kustomize.sh"

# Configure e2e tests
export GINKGO_NODES=3
export GINKGO_ARGS="--fail-fast" # Other ginkgo args that need to be appended to the command.
ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
mkdir -p "${ARTIFACTS}/logs/"

# Verify the required Environment Variables are present.
: "${GOOGLE_APPLICATION_CREDENTIALS:?Environment variable empty or not defined.}"

export GCP_REGION=${GCP_REGION:-"us-east4"}
export TEST_NAME=${CLUSTER_NAME:-"capg-${RANDOM}"}
export GCP_NETWORK_NAME=${GCP_NETWORK_NAME:-"${TEST_NAME}-mynetwork"}
GCP_B64ENCODED_CREDENTIALS=$(base64 "$GOOGLE_APPLICATION_CREDENTIALS" | tr -d '\n')
export GCP_B64ENCODED_CREDENTIALS
export KUBERNETES_MAJOR_VERSION="1"
export KUBERNETES_MINOR_VERSION="27"
export KUBERNETES_PATCH_VERSION="3"
export KUBERNETES_VERSION="v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}"
# using prebuilt image from image-builder project the image is built everyday and the job is available here https://prow.k8s.io/?job=periodic-image-builder-gcp-all-nightly
export IMAGE_ID="projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-2204-${KUBERNETES_VERSION//[.+]/-}-nightly"

init_image() {
  if [[ "${REUSE_OLD_IMAGES:-false}" == "true" ]]; then
    image=$(gcloud compute images list --project "$GCP_PROJECT" \
      --no-standard-images --filter="family:capi-ubuntu-2204-k8s-v${KUBERNETES_MAJOR_VERSION}-${KUBERNETES_MINOR_VERSION}" --format="table[no-heading](name)")
    if [[ -n "$image" ]]; then
      return
    fi
  fi

  cat << EOF > "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi/override.json"
{
  "build_timestamp": "0",
  "kubernetes_series": "v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}",
  "kubernetes_semver": "${KUBERNETES_VERSION}",
  "kubernetes_deb_version": "${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}-00",
  "kubernetes_rpm_version": "${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}-0"
}
EOF

  if [[ $EUID -ne 0 ]]; then
    (cd "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi" && \
      GCP_PROJECT_ID=$GCP_PROJECT \
      GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
      PACKER_VAR_FILES=override.json \
      make deps-gce build-gce-ubuntu-2004)
  else
    # assume we are running in the CI environment as root
    # Add a user for ansible to work properly
    groupadd -r packer && useradd -m -s /bin/bash -r -g packer packer
    chown -R packer:packer /home/prow/go/src/sigs.k8s.io/image-builder
    # use the packer user to run the build
    su - packer -c "bash -c 'cd /home/prow/go/src/sigs.k8s.io/image-builder/images/capi && PATH=$PATH:~packer/.local/bin:/home/prow/go/src/sigs.k8s.io/image-builder/images/capi/.local/bin GCP_PROJECT_ID=$GCP_PROJECT GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS PACKER_VAR_FILES=override.json make deps-gce build-gce-ubuntu-2204'"
  fi

  filter="name~cluster-api-ubuntu-2204-${KUBERNETES_VERSION//[.+]/-}"
  image_id=$(gcloud compute images list --project "$GCP_PROJECT" \
    --no-standard-images --filter="${filter}" --format="table[no-heading](name)")
  if [[ -z "$image_id" ]]; then
    echo "unable to find image using : $filter $GCP_PROJECT ... bailing out!"
    exit 1
  fi

  export IMAGE_ID="projects/${GCP_PROJECT}/global/images/${image_id}"
}


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
  fi

  gcloud compute firewall-rules list --project "$GCP_PROJECT"
  gcloud compute networks list --project="${GCP_PROJECT}"
  gcloud compute networks describe "${GCP_NETWORK_NAME}" --project="${GCP_PROJECT}"

  gcloud compute routers create "${TEST_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --network="${GCP_NETWORK_NAME}"
  gcloud compute routers nats create "${TEST_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${TEST_NAME}-myrouter" \
    --nat-all-subnet-ip-ranges --auto-allocate-nat-external-ips
}


cleanup() {
  # Force a cleanup of cluster api created resources using gcloud commands
  (gcloud compute forwarding-rules list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute forwarding-rules delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute target-tcp-proxies list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute target-tcp-proxies delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute backend-services list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute backend-services delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute health-checks list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute health-checks delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute instances list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute instances delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute instance-groups list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute instance-groups unmanaged delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute firewall-rules list --project "$GCP_PROJECT" | grep capg-e2e \
       | awk '{print "gcloud compute firewall-rules delete --project '"$GCP_PROJECT"' --quiet " $1 "\n"}' \
       | bash) || true

  # cleanup the networks
  gcloud compute routers nats delete "${TEST_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${TEST_NAME}-myrouter" --quiet || true
  gcloud compute routers delete "${TEST_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --quiet || true

  if [[ ${GCP_NETWORK_NAME} != "default" ]]; then
    (gcloud compute firewall-rules list --project "$GCP_PROJECT" | grep "$GCP_NETWORK_NAME" \
         | awk '{print "gcloud compute firewall-rules delete --project '"$GCP_PROJECT"' --quiet " $1 "\n"}' \
         | bash) || true
    gcloud compute networks delete --project="${GCP_PROJECT}" \
      --quiet "${GCP_NETWORK_NAME}" || true
  fi

  if [[ -n "${SKIP_INIT_IMAGE:-}" ]]; then
    echo "Skipping GCP image deletion..."
  else
    # removing the image created
    gcloud compute images delete "${image_id}" --project "${GCP_PROJECT}" --quiet || true
  fi

  # stop boskos heartbeat
  [[ -z ${HEART_BEAT_PID:-} ]] || kill -9 "${HEART_BEAT_PID}" || true
}

# our exit handler (trap)
exit-handler() {
  unset KUBECONFIG
  cleanup
}

# setup gcp network, build image run the e2es
main() {
  # skip the build image by default for CI
  # locally if want to build the image pass the flag --init-image
  SKIP_INIT_IMAGE="1"

  for arg in "$@"
  do
    if [[ "${arg}" == "--verbose" ]]; then
      set -o xtrace
    fi
    if [[ "${arg}" == "--init-image" ]]; then
      unset SKIP_INIT_IMAGE
    fi
    if [[ "${arg}" == "--build-image-only" ]]; then
      BUILD_IMAGE_ONLY="1"
    fi
  done

  # If BOSKOS_HOST is set then acquire an GCP account from Boskos.
  if [[ -n "${BOSKOS_HOST:-}" ]]; then
    # Check out the account from Boskos and store the produced environment
    # variables in a temporary file.
    account_env_var_file="$(mktemp)"
    python3 hack/checkout_account.py 1>"${account_env_var_file}"
    checkout_account_status="${?}"

    # If the checkout process was a success then load the account's
    # environment variables into this process.
    # shellcheck disable=SC1090
    [[ "${checkout_account_status}" = "0" ]] && . "${account_env_var_file}"

    # Always remove the account environment variable file. It contains
    # sensitive information.
    rm -f "${account_env_var_file}"

    if [[ ! "${checkout_account_status}" = "0" ]]; then
      echo "error getting account from boskos" 1>&2
      exit "${checkout_account_status}"
    fi

    # run the heart beat process to tell boskos that we are still
    # using the checked out account periodically
    python3 -u hack/heartbeat_account.py >> "${ARTIFACTS}/logs/boskos.log" 2>&1 &
    # shellcheck disable=SC2116
    HEART_BEAT_PID=$(echo $!)
  fi

  if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS}" ]]; then
    cat <<EOF
GOOGLE_APPLICATION_CREDENTIALS is not set.
Please set this to the path of the service account used to run this script.
EOF
    return 2
  else
    gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
  fi
  if [[ -z "$GCP_PROJECT" ]]; then
    GCP_PROJECT=$(jq -r .project_id "${GOOGLE_APPLICATION_CREDENTIALS}")
    cat <<EOF
GCP_PROJECT is not set. Using project_id $GCP_PROJECT
EOF
  fi
  if [[ -z "$GCP_REGION" ]]; then
    cat <<EOF
GCP_REGION is not set.
Please specify which the GCP region to use.
EOF
    return 2
  fi

  SKIP_CLEANUP=${SKIP_CLEANUP:-""}
  if [[ -z "${SKIP_CLEANUP}" ]]; then
    trap exit-handler EXIT
  fi

  if [[ -n "${SKIP_INIT_IMAGE:-}" ]]; then
    echo "Skipping GCP image initialization..."
  else
    init_image
    if [[ -n "${BUILD_IMAGE_ONLY:-}" ]]; then
      exit 0
    fi
  fi

  # Initialize the necessary network requirements
  if [[ -n "${SKIP_INIT_NETWORK:-}" ]]; then
    echo "Skipping network initialization..."
  else
    init_networks
  fi

  make test-e2e
  test_status="${?}"
  echo TESTSTATUS
  echo "${test_status}"

  # If Boskos is being used then release the GCP project back to Boskos.
  [[ -z "${BOSKOS_HOST:-}" ]] || hack/checkin_account.py >> "${ARTIFACTS}/logs/boskos.log" 2>&1
}

main "$@"
exit "${test_status}"
