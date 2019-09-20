#!/usr/bin/env bash

# Copyright 2019 The Kubernetes Authors.
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

# hack script for running a cluster-api-provider-gcp e2e

set -o errexit -o nounset -o pipefail -o xtrace

GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-""}
GCP_PROJECT=${GCP_PROJECT:-""}
GCP_REGION=${GCP_REGION:-"us-east4"}
CLUSTER_NAME=${CLUSTER_NAME:-"test1"}

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# our exit handler (trap)
cleanup() {
  # always attempt to dump logs
  kind "export" logs --name="clusterapi" "${ARTIFACTS}/logs" || true
  # KIND_IS_UP is true once we: kind create
  if [[ "${KIND_IS_UP:-}" = true ]]; then
    kubectl \
      --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      delete cluster test1 || true
    kubectl \
      --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      wait --for=delete cluster/test1 || true
    make kind-reset || true
  fi
  # clean up e2e.test symlink
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && rm -f _output/bin/e2e.test) || true
  # remove our tempdir
  # NOTE: this needs to be last, or it will prevent kind delete
  if [[ -n "${TMP_DIR:-}" ]]; then
    rm -rf "${TMP_DIR}"
  fi
}

init_image() {
  image=$(gcloud compute images list --project $GCP_PROJECT \
    --no-standard-images --filter="family:capi-ubuntu-1804-k8s" --format="table[no-heading](name)")
  if [[ -z "$image" ]]; then
      if ! command -v packer &> /dev/null; then
        hostos=$(go env GOHOSTOS)
        hostarch=$(go env GOHOSTARCH)
        version="1.4.3"
        url="https://releases.hashicorp.com/packer/${version}/packer_${version}_${hostos}_${hostarch}.zip"
        echo "Downloading packer from $url"
        wget -O packer.zip $url  && \
          unzip packer.zip && \
          rm packer.zip && \
          mv packer "$(go env GOPATH)/bin"
      fi
      (cd "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi" && \
      GCP_PROJECT_ID=$GCP_PROJECT GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
      make build-gce-default)
  fi
}

# build kubernetes / node image, e2e binaries
build() {
  # possibly enable bazel build caching before building kubernetes
  if [[ "${BAZEL_REMOTE_CACHE_ENABLED:-false}" == "true" ]]; then
    create_bazel_cache_rcs.sh || true
  fi

  pushd "$(go env GOPATH)/src/k8s.io/kubernetes"

  # make sure we have e2e requirements
  bazel build //cmd/kubectl //test/e2e:e2e.test //vendor/github.com/onsi/ginkgo/ginkgo

  # ensure the e2e script will find our binaries ...
  mkdir -p "${PWD}/_output/bin/"
  cp "${PWD}/bazel-bin/test/e2e/e2e.test" "${PWD}/_output/bin/e2e.test"
  PATH="$(dirname "$(find "${PWD}/bazel-bin/" -name kubectl -type f)"):${PATH}"
  export PATH

  # attempt to release some memory after building
  sync || true
  echo 1 > /proc/sys/vm/drop_caches || true

  popd
}

# generate manifests needed for creating the GCP cluster to run the tests
generate_manifests() {
GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
	GCP_REGION=$GCP_REGION \
	GCP_PROJECT=$GCP_PROJECT \
	CLUSTER_NAME=$CLUSTER_NAME \
	make generate-examples
}

# up a cluster with kind
create_cluster() {
  # actually create the cluster
  KIND_IS_UP=true
  make create-cluster

  # Wait till all machines are running
  while true; do
    read running total <<< $(kubectl get machines --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
     -o json | jq -r '.items[].status.phase' | awk '/running/ {count++} END{print count " " NR}') ;
    if [[ $total == "5" && $running == "5" ]]; then
      return
    fi
    timestamp=$(date +"[%H:%M:%S]")
    echo "$timestamp Total machines : $total / Running : $running .. waiting for 10 seconds"
    sleep 10
  done
}

# run e2es with kubetest
run_tests() {
  # export the KUBECONFIG
  KUBECONFIG="${PWD}/kubeconfig"
  export KUBECONFIG

  # ginkgo regexes
  SKIP="${SKIP:-}"
  FOCUS="${FOCUS:-"\\[Conformance\\]"}"
  # if we set PARALLEL=true, skip serial tests set --ginkgo-parallel
  if [[ "${PARALLEL:-false}" == "true" ]]; then
    export GINKGO_PARALLEL=y
    if [[ -z "${SKIP}" ]]; then
      SKIP="\\[Serial\\]"
    else
      SKIP="\\[Serial\\]|${SKIP}"
    fi
  fi

  # get the number of worker nodes
  # TODO(bentheelder): this is kinda gross
  NUM_NODES="$(kubectl get nodes --kubeconfig=$KUBECONFIG \
    -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}' \
    | grep -cv "node-role.kubernetes.io/master" )"

  # wait for all the nodes to be ready
  kubectl wait --for=condition=Ready node --kubeconfig=$KUBECONFIG --all

  # setting this env prevents ginkg e2e from trying to run provider setup
  export KUBERNETES_CONFORMANCE_TEST="y"
  # run the tests
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && ./hack/ginkgo-e2e.sh \
    '--provider=skeleton' "--num-nodes=${NUM_NODES}" \
    "--ginkgo.focus=${FOCUS}" "--ginkgo.skip=${SKIP}" \
    "--report-dir=${ARTIFACTS}" '--disable-log-dump=true')

  unset KUBECONFIG
  unset KUBERNETES_CONFORMANCE_TEST
}

# setup kind, build kubernetes, create a cluster, run the e2es
main() {
  if [[ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
    cat <<EOF
$GOOGLE_APPLICATION_CREDENTIALS is not set.
Please set this to the path of the service account used to run this script.
EOF
    return 2
  else
    gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
  fi
  if [[ -z "$GCP_PROJECT" ]]; then
    GCP_PROJECT=$(cat ${GOOGLE_APPLICATION_CREDENTIALS} | jq -r .project_id)
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

  # create temp dir and setup cleanup
  TMP_DIR=$(mktemp -d)
  trap cleanup EXIT
  # ensure artifacts exists when not in CI
  ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
  export ARTIFACTS
  mkdir -p "${ARTIFACTS}"

  source "${REPO_ROOT}/hack/ensure-go.sh"
  source "${REPO_ROOT}/hack/ensure-kind.sh"

  # now build and run the cluster and tests
  init_image
  build
  generate_manifests
  create_cluster
  run_tests
}

main
