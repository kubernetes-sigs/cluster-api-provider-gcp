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

set -o errexit -o nounset -o pipefail

GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-""}
GCP_PROJECT=${GCP_PROJECT:-""}
GCP_REGION=${GCP_REGION:-"us-east4"}
CLUSTER_NAME=${CLUSTER_NAME:-"test1"}
NETWORK_NAME=${NETWORK_NAME:-"${CLUSTER_NAME}-mynetwork"}

TIMESTAMP=$(date +"%Y-%m-%dT%H:%M:%SZ")

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# dump logs from kind and all the nodes
dump-logs() {
  # always attempt to dump logs
  kind "export" logs --name="clusterapi" "${ARTIFACTS}/logs" || true

  gcloud compute instances list --project "${GCP_PROJECT}"

  for node_name in $(gcloud compute instances list --project "${GCP_PROJECT}" --format='value(name)')
  do
    node_zone=$(gcloud compute instances list --project "${GCP_PROJECT}" --filter="name:(${node_name})" --format='value(zone)')
    echo "collecting logs from ${node_name} in zone ${node_zone}"
    dir="${ARTIFACTS}/logs/${node_name}"
    mkdir -p ${dir}

    gcloud compute instances get-serial-port-output --project "${GCP_PROJECT}" \
      --zone "${node_zone}" --port 1 "${node_name}" > "${dir}/serial-1.log" || true

    ssh-to-node "${node_name}" "${node_zone}" "sudo chmod -R a+r /var/log" || true
    gcloud compute scp --recurse --project "${GCP_PROJECT}" --zone "${node_zone}" \
      "${node_name}:/var/log/cloud-init.log" "${node_name}:/var/log/cloud-init-output.log" \
      "${node_name}:/var/log/pods" "${node_name}:/var/log/containers" \
      "${dir}" || true

    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --output=short-precise -k" > "${dir}/kern.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --output=short-precise" > "${dir}/systemd.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo crictl version && sudo crictl info" > "${dir}/containerd.info" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u kubelet.service" > "${dir}/kubelet.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u containerd.service" > "${dir}/containerd.log" || true
  done

  gcloud logging read --order=asc \
    --format='table(timestamp,jsonPayload.resource.name,jsonPayload.event_subtype)' \
    --project "${GCP_PROJECT}" \
    "timestamp >= \"${TIMESTAMP}\"" \
     > "${ARTIFACTS}/logs/activity.log" || true
}

# our exit handler (trap)
cleanup() {
  # dump all the logs
  dump-logs

  # KIND_IS_UP is true once we: kind create
  if [[ "${KIND_IS_UP:-}" = true ]]; then
    timeout 600 kubectl \
      --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      delete cluster test1 || true
     timeout 600 kubectl \
      --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      wait --for=delete cluster/test1 || true
    make kind-reset || true
  fi
  # clean up e2e.test symlink
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && rm -f _output/bin/e2e.test) || true

  # Force a cleanup of cluster api created resources using gcloud commands
  gcloud compute forwarding-rules delete --project $GCP_PROJECT --global $CLUSTER_NAME-apiserver --quiet || true
  gcloud compute target-tcp-proxies delete --project $GCP_PROJECT $CLUSTER_NAME-apiserver --quiet || true
  gcloud compute backend-services delete --project $GCP_PROJECT --global $CLUSTER_NAME-apiserver --quiet || true
  gcloud compute health-checks delete --project $GCP_PROJECT $CLUSTER_NAME-apiserver --quiet || true
  (gcloud compute instances list --project $GCP_PROJECT | grep $CLUSTER_NAME \
       | awk '{print "gcloud compute instances delete --project '$GCP_PROJECT' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute instance-groups list --project $GCP_PROJECT | grep $CLUSTER_NAME \
       | awk '{print "gcloud compute instance-groups unmanaged delete --project '$GCP_PROJECT' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute firewall-rules list --project $GCP_PROJECT | grep $CLUSTER_NAME \
       | awk '{print "gcloud compute firewall-rules delete --project '$GCP_PROJECT' --quiet " $1 "\n"}' \
       | bash) || true

  # cleanup the networks
  gcloud compute routers nats delete "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter" --quiet || true
  gcloud compute routers delete "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --quiet || true

  if [[ ${NETWORK_NAME} != "default" ]]; then
    (gcloud compute firewall-rules list --project $GCP_PROJECT | grep $NETWORK_NAME \
         | awk '{print "gcloud compute firewall-rules delete --project '$GCP_PROJECT' --quiet " $1 "\n"}' \
         | bash) || true
    gcloud compute networks delete --project="${GCP_PROJECT}" \
      --quiet "${NETWORK_NAME}" || true
  fi

  # remove our tempdir
  # NOTE: this needs to be last, or it will prevent kind delete
  if [[ -n "${TMP_DIR:-}" ]]; then
    rm -rf "${TMP_DIR}" || true
  fi
}

# SSH to a node by name ($1) and run a command ($2).
function ssh-to-node() {
  local node="$1"
  local zone="$2"
  local cmd="$3"

  # ensure we have an IP to connect to
  gcloud compute --project "${GCP_PROJECT}" instances add-access-config --zone "${zone}" "${node}" || true

  # Loop until we can successfully ssh into the box
  for try in {1..5}; do
    if gcloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" \
      --project "${GCP_PROJECT}" --zone "${zone}" "${node}" --command "echo test > /dev/null"; then
      break
    fi
    sleep 5
  done
  # Then actually try the command.
  gcloud compute ssh --ssh-flag="-o LogLevel=quiet" --ssh-flag="-o ConnectTimeout=30" \
    --project "${GCP_PROJECT}" --zone "${zone}" "${node}" --command "${cmd}"
}

init_image() {
  image=$(gcloud compute images list --project $GCP_PROJECT \
    --no-standard-images --filter="family:capi-ubuntu-1804-k8s-v1-16" --format="table[no-heading](name)")
  if [[ -z "$image" ]]; then
      if ! command -v ansible &> /dev/null; then
        if [[ $EUID -ne 0 ]]; then
          echo "Please install ansible and try again."
          exit 1
        else
          # we need pip to install ansible
          curl -L https://bootstrap.pypa.io/get-pip.py -o get-pip.py
          python get-pip.py --user
          rm -f get-pip.py

          # install ansible needed by packer
          version="2.8.5"
          python -m pip install "ansible==${version}"
        fi
      fi
      if ! command -v packer &> /dev/null; then
        hostos=$(go env GOHOSTOS)
        hostarch=$(go env GOHOSTARCH)
        version="1.4.3"
        url="https://releases.hashicorp.com/packer/${version}/packer_${version}_${hostos}_${hostarch}.zip"
        echo "Downloading packer from $url"
        wget --quiet -O packer.zip $url  && \
          unzip packer.zip && \
          rm packer.zip && \
          ln -s $PWD/packer /usr/local/bin/packer
      fi
      (cd "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi" && \
        sed -i 's/1\.15\.4/1.16.0/' packer/config/kubernetes.json && \
        sed -i 's/1\.15/1.16/' packer/config/kubernetes.json)
      if [[ $EUID -ne 0 ]]; then
        (cd "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi" && \
          GCP_PROJECT_ID=$GCP_PROJECT GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
          make build-gce-default)
      else
        # assume we are running in the CI environment as root
        # Add a user for ansible to work properly
        groupadd -r packer && useradd -m -s /bin/bash -r -g packer packer
        # use the packer user to run the build
        su - packer -c "bash -c 'cd /go/src/sigs.k8s.io/image-builder/images/capi && GCP_PROJECT_ID=$GCP_PROJECT GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS make build-gce-default'"
      fi
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
  if ! command -v kustomize >/dev/null 2>&1; then
    GO111MODULE=on go install sigs.k8s.io/kustomize/v3/cmd/kustomize
  fi

  GCP_PROJECT=$GCP_PROJECT \
    make docker-build
  GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
    GCP_REGION=$GCP_REGION \
    GCP_PROJECT=$GCP_PROJECT \
    CLUSTER_NAME=$CLUSTER_NAME \
    NETWORK_NAME=$NETWORK_NAME \
    KUBERNETES_VERSION="v1.16.0" \
    make generate-examples
}

# up a cluster with kind
create_cluster() {
  # actually create the cluster
  KIND_IS_UP=true

  # Load the newly built image into kind and start the cluster
  LOAD_IMAGE="gcr.io/${GCP_PROJECT}/cluster-api-gcp-controller-amd64:dev" make create-cluster

  # Wait till all machines are running (bail out at 30 mins)
  attempt=0
  while true; do
    kubectl get machines --kubeconfig=$(kind get kubeconfig-path --name="clusterapi")
    read running total <<< $(kubectl get machines --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /running/{count++} END{print count " " NR}') ;
    if [[ $total == "5" && $running == "5" ]]; then
      return 0
    fi
    read failed total <<< $(kubectl get machines --kubeconfig=$(kind get kubeconfig-path --name="clusterapi") \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /failed/{count++} END{print count " " NR}') ;
    if [[ ! $failed -eq 0 ]]; then
      echo "$failed machines (out of $total) in cluster failed ... bailing out"
      exit 1
    fi
    timestamp=$(date +"[%H:%M:%S]")
    if [ $attempt -gt 180 ]; then
      echo "cluster did not start in 30 mins ... bailing out!"
      exit 1
    fi
    echo "$timestamp Total machines : $total / Running : $running .. waiting for 10 seconds"
    sleep 10
    attempt=$((attempt+1))
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
  kubectl wait --for=condition=Ready node --kubeconfig=$KUBECONFIG --all || true

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

# initialize a router and cloud NAT
init_networks() {
  if [[ ${NETWORK_NAME} != "default" ]]; then
    gcloud compute networks create --project $GCP_PROJECT ${NETWORK_NAME} --subnet-mode auto --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-http --project $GCP_PROJECT \
      --allow tcp:80 --network ${NETWORK_NAME} --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-https --project $GCP_PROJECT \
      --allow tcp:443 --network ${NETWORK_NAME} --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-icmp --project $GCP_PROJECT \
      --allow icmp --network ${NETWORK_NAME} --priority 65534 --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-internal --project $GCP_PROJECT \
      --allow "tcp:0-65535,udp:0-65535,icmp" --network ${NETWORK_NAME} --priority 65534 --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-rdp --project $GCP_PROJECT \
      --allow "tcp:3389" --network ${NETWORK_NAME} --priority 65534 --quiet
    gcloud compute firewall-rules create ${NETWORK_NAME}-allow-ssh --project $GCP_PROJECT \
      --allow "tcp:22" --network ${NETWORK_NAME} --priority 65534 --quiet
  fi

  gcloud compute firewall-rules list --project $GCP_PROJECT
  gcloud compute networks list --project="${GCP_PROJECT}"
  gcloud compute networks describe ${NETWORK_NAME} --project="${GCP_PROJECT}"

  gcloud compute routers create "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" \
    --region="${GCP_REGION}" --network=${NETWORK_NAME}
  gcloud compute routers nats create "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" \
    --router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter" \
    --nat-all-subnet-ip-ranges --auto-allocate-nat-external-ips
}

# setup kind, build kubernetes, create a cluster, run the e2es
main() {
  if [[ ${1:-} == "--verbose" ]]; then
     set -o xtrace
  fi
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
  SKIP_CLEANUP=${SKIP_CLEANUP:-""}
  if [[ -z "${SKIP_CLEANUP}" ]]; then
    trap cleanup EXIT
  fi
  # ensure artifacts exists when not in CI
  ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"
  export ARTIFACTS
  mkdir -p "${ARTIFACTS}/logs"

  source "${REPO_ROOT}/hack/ensure-go.sh"
  source "${REPO_ROOT}/hack/ensure-kind.sh"

  # now build and run the cluster and tests
  init_networks
  build
  generate_manifests
  init_image
  create_cluster
  SKIP_RUN_TESTS=${SKIP_RUN_TESTS:-""}
  if [[ -z "${SKIP_RUN_TESTS}" ]]; then
    run_tests
  fi
}

main $@
