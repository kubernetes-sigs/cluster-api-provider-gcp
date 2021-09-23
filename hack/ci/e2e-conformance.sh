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
CAPG_WORKER_CLUSTER_KUBECONFIG=${CAPG_WORKER_CLUSTER_KUBECONFIG:-"/tmp/kubeconfig"}
GCP_NETWORK_NAME=${GCP_NETWORK_NAME:-"${CLUSTER_NAME}-mynetwork"}
KUBERNETES_MAJOR_VERSION="1"
KUBERNETES_MINOR_VERSION="20"
KUBERNETES_PATCH_VERSION="9"
KUBERNETES_VERSION="v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}"
CONTROL_PLANE_MACHINE_COUNT=1
WORKER_MACHINE_COUNT=5
TOTAL_MACHINE_COUNT=$((CONTROL_PLANE_MACHINE_COUNT+WORKER_MACHINE_COUNT))

TIMESTAMP=$(date +"%Y-%m-%dT%H:%M:%SZ")

ARTIFACTS="${ARTIFACTS:-${PWD}/_artifacts}"

# dump logs from kind and all the nodes
dump-logs() {
  # log version information
  echo "=== versions ==="
  echo "kind : $(kind version)" || true
  echo "bootstrap cluster:"
  kubectl version || true
  echo "deployed cluster:"
  kubectl --kubeconfig="${CAPG_WORKER_CLUSTER_KUBECONFIG}" version || true
  echo ""

  # dump all the info from the CAPI related CRDs
  kubectl --context=kind-clusterapi get \
  clusters,gcpclusters,machines,gcpmachines,kubeadmconfigs,machinedeployments,gcpmachinetemplates,kubeadmconfigtemplates,machinesets,kubeadmcontrolplanes \
  --all-namespaces -o yaml >> "${ARTIFACTS}/logs/capg.info" || true

  # dump images info
  echo "images in docker" >> "${ARTIFACTS}/logs/images.info"
  docker images >> "${ARTIFACTS}/logs/images.info"
  echo "images from bootstrap using containerd CLI" >> "${ARTIFACTS}/logs/images.info"
  docker exec clusterapi-control-plane ctr -n k8s.io images list >> "${ARTIFACTS}/logs/images.info" || true
  echo "images in bootstrap cluster using kubectl CLI" >> "${ARTIFACTS}/logs/images.info"
  (kubectl get pods --all-namespaces -o json \
   | jq --raw-output '.items[].spec.containers[].image' | sort)  >> "${ARTIFACTS}/logs/images.info" || true
  echo "images in deployed cluster using kubectl CLI" >> "${ARTIFACTS}/logs/images.info"
  (kubectl --kubeconfig="${CAPG_WORKER_CLUSTER_KUBECONFIG}" get pods --all-namespaces -o json \
   | jq --raw-output '.items[].spec.containers[].image' | sort)  >> "${ARTIFACTS}/logs/images.info" || true

  # dump cluster info for kind
  kubectl cluster-info dump > "${ARTIFACTS}/logs/kind-cluster.info" || true

  # dump cluster info for kind
  echo "=== gcloud compute instances list ===" >> "${ARTIFACTS}/logs/capg-cluster.info" || true
  gcloud compute instances list --project "${GCP_PROJECT}" >> "${ARTIFACTS}/logs/capg-cluster.info" || true
  echo "=== cluster-info dump ===" >> "${ARTIFACTS}/logs/capg-cluster.info" || true
  kubectl --kubeconfig="${CAPG_WORKER_CLUSTER_KUBECONFIG}" cluster-info dump >> "${ARTIFACTS}/logs/capg-cluster.info" || true

  # export all logs from kind
  kind "export" logs --name="clusterapi" "${ARTIFACTS}/logs" || true

  for node_name in $(gcloud compute instances list --filter="zone~'${GCP_REGION}-.*'" --project "${GCP_PROJECT}" --format='value(name)')
  do
    node_zone=$(gcloud compute instances list --project "${GCP_PROJECT}" --filter="name:(${node_name})" --format='value(zone)')
    echo "collecting logs from ${node_name} in zone ${node_zone}"
    dir="${ARTIFACTS}/logs/${node_name}"
    mkdir -p "${dir}"

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
    ssh-to-node "${node_name}" "${node_zone}" "sudo cat /etc/containerd/config.toml" > "${dir}/containerd-config.toml" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u kubelet.service" > "${dir}/kubelet.log" || true
    ssh-to-node "${node_name}" "${node_zone}" "sudo journalctl --no-pager -u containerd.service" > "${dir}/containerd.log" || true
  done

  gcloud logging read --order=asc \
    --format='table(timestamp,jsonPayload.resource.name,jsonPayload.event_subtype)' \
    --project "${GCP_PROJECT}" \
    "timestamp >= \"${TIMESTAMP}\"" \
     > "${ARTIFACTS}/logs/activity.log" || true
}

# cleanup all resources we use
cleanup() {
  # KIND_IS_UP is true once we: kind create
  if [[ "${KIND_IS_UP:-}" = true ]]; then
    timeout 600 kubectl \
      delete cluster "${CLUSTER_NAME}" || true
    timeout 600 kubectl \
      wait --for=delete cluster/"${CLUSTER_NAME}" || true
    make kind-reset || true
  fi
  # clean up e2e.test symlink
  (cd "$(go env GOPATH)/src/k8s.io/kubernetes" && rm -f _output/bin/e2e.test) || true

  # Force a cleanup of cluster api created resources using gcloud commands
  gcloud compute forwarding-rules delete --project "$GCP_PROJECT" --global "$CLUSTER_NAME"-apiserver --quiet || true
  gcloud compute target-tcp-proxies delete --project "$GCP_PROJECT" "$CLUSTER_NAME"-apiserver --quiet || true
  gcloud compute backend-services delete --project "$GCP_PROJECT" --global "$CLUSTER_NAME"-apiserver --quiet || true
  gcloud compute health-checks delete --project "$GCP_PROJECT" "$CLUSTER_NAME"-apiserver --quiet || true
  (gcloud compute instances list --project "$GCP_PROJECT" | grep "$CLUSTER_NAME" \
       | awk '{print "gcloud compute instances delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute instance-groups list --project "$GCP_PROJECT" | grep "$CLUSTER_NAME" \
       | awk '{print "gcloud compute instance-groups unmanaged delete --project '"$GCP_PROJECT"' --quiet " $1 " --zone " $2 "\n"}' \
       | bash) || true
  (gcloud compute firewall-rules list --project "$GCP_PROJECT" | grep "$CLUSTER_NAME" \
       | awk '{print "gcloud compute firewall-rules delete --project '"$GCP_PROJECT"' --quiet " $1 "\n"}' \
       | bash) || true

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

  if [[ "${REUSE_OLD_IMAGES:-false}" == "false" ]]; then
    (gcloud compute images list --project "$GCP_PROJECT" \
      --no-standard-images --filter="family:capi-ubuntu-1804-k8s-v${KUBERNETES_MAJOR_VERSION}-${KUBERNETES_MINOR_VERSION}" --format="table[no-heading](name)" \
         | awk '{print "gcloud compute images delete --project '"$GCP_PROJECT"' --quiet " $1 "\n"}' \
         | bash) || true
  fi

  # remove our tempdir
  # NOTE: this needs to be last, or it will prevent kind delete
  if [[ -n "${TMP_DIR:-}" ]]; then
    rm -rf "${TMP_DIR}" || true
  fi
}

# our exit handler (trap)
exit-handler() {
  unset KUBECONFIG
  dump-logs
  cleanup
}

# SSH to a node by name ($1) and run a command ($2).
function ssh-to-node() {
  local node="$1"
  local zone="$2"
  local cmd="$3"

  # ensure we have an IP to connect to
  gcloud compute instances add-access-config "${node}" --project "${GCP_PROJECT}" --zone "${zone}" || true

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
  if [[ "${REUSE_OLD_IMAGES:-false}" == "true" ]]; then
    image=$(gcloud compute images list --project "$GCP_PROJECT" \
      --no-standard-images --filter="family:capi-ubuntu-1804-k8s-v${KUBERNETES_MAJOR_VERSION}-${KUBERNETES_MINOR_VERSION}" --format="table[no-heading](name)")
    if [[ -n "$image" ]]; then
      return
    fi
  fi

  if [[ -n ${CI_VERSION:-} ]]; then
    cat << EOF > "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi/override.json"
{
  "build_timestamp": "0",
  "kubernetes_source_type": "http",
  "kubernetes_cni_source_type": "http",
  "kubernetes_http_source": "https://dl.k8s.io/ci",
  "kubernetes_series": "v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}",
  "kubernetes_semver": "${KUBERNETES_VERSION}"
}
EOF
  else
    cat << EOF > "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi/override.json"
{
  "build_timestamp": "0",
  "kubernetes_series": "v${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}",
  "kubernetes_semver": "${KUBERNETES_VERSION}",
  "kubernetes_deb_version": "${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}-00",
  "kubernetes_rpm_version": "${KUBERNETES_MAJOR_VERSION}.${KUBERNETES_MINOR_VERSION}.${KUBERNETES_PATCH_VERSION}-0"
}
EOF
  fi

  if [[ $EUID -ne 0 ]]; then
    (cd "$(go env GOPATH)/src/sigs.k8s.io/image-builder/images/capi" && \
      GCP_PROJECT_ID=$GCP_PROJECT \
      GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS \
      PACKER_VAR_FILES=override.json \
      make deps-gce build-gce-ubuntu-1804)
  else
    # assume we are running in the CI environment as root
    # Add a user for ansible to work properly
    groupadd -r packer && useradd -m -s /bin/bash -r -g packer packer
    chown -R packer:packer /home/prow/go/src/sigs.k8s.io/image-builder
    # use the packer user to run the build
    su - packer -c "bash -c 'cd /home/prow/go/src/sigs.k8s.io/image-builder/images/capi && PATH=$PATH:~packer/.local/bin:/home/prow/go/src/sigs.k8s.io/image-builder/images/capi/.local/bin GCP_PROJECT_ID=$GCP_PROJECT GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS PACKER_VAR_FILES=override.json make deps-gce build-gce-ubuntu-1804'"
  fi
}

# build Kubernetes E2E binaries
build_k8s() {
  # possibly enable bazel build caching before building kubernetes
  if [[ "${BAZEL_REMOTE_CACHE_ENABLED:-false}" == "true" ]]; then
    create_bazel_cache_rcs.sh || true
  fi

  pushd "$(go env GOPATH)/src/k8s.io/kubernetes"

  # make sure we have e2e requirements
  make WHAT="test/e2e/e2e.test vendor/github.com/onsi/ginkgo/ginkgo cmd/kubectl"

  # ensure the e2e script will find our binaries ...
  PATH="$(dirname "$(find "${PWD}/bazel-bin/" -name kubectl -type f)"):${PATH}"
  export PATH

  # attempt to release some memory after building
  sync || true
  echo 1 > /proc/sys/vm/drop_caches || true

  popd
}

# up a cluster with kind
create_cluster() {
  # actually create the cluster
  KIND_IS_UP=true

  if [[ -n "${SKIP_INIT_IMAGE:-}" ]]; then
    echo "Using prebuilt node image"
    export IMAGE_TO_USE="projects/k8s-staging-cluster-api-gcp/global/images/cluster-api-ubuntu-1804-${KUBERNETES_VERSION//[.+]/-}-nightly"
  else
    filter="name~cluster-api-ubuntu-1804-${KUBERNETES_VERSION//[.+]/-}"
    image_id=$(gcloud compute images list --project "$GCP_PROJECT" \
      --no-standard-images --filter="${filter}" --format="table[no-heading](name)")
    if [[ -z "$image_id" ]]; then
      echo "unable to find image using : $filter $GCP_PROJECT ... bailing out!"
      exit 1
    fi
    export IMAGE_TO_USE="projects/${GCP_PROJECT}/global/images/${image_id}"
  fi

  tracestate="$(shopt -po xtrace)"
  set +o xtrace

  # Load the newly built image into kind and start the cluster
  (GCP_REGION=${GCP_REGION} \
  GCP_PROJECT=${GCP_PROJECT} \
  CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT} \
  WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT} \
  KUBERNETES_VERSION=${KUBERNETES_VERSION} \
  GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2 \
  GCP_NODE_MACHINE_TYPE=n1-standard-2 \
  GCP_NETWORK_NAME=${GCP_NETWORK_NAME} \
  GCP_B64ENCODED_CREDENTIALS=$(base64 -w0 "$GOOGLE_APPLICATION_CREDENTIALS") \
  CLUSTER_NAME="${CLUSTER_NAME}" \
  CI_VERSION=${CI_VERSION:-} \
  IMAGE_ID="${IMAGE_TO_USE}" \
    make create-cluster)

  eval "$tracestate"

  # Wait till all machines are running (bail out at 30 mins)
  attempt=0
  while true; do
    kubectl get machines --context=kind-clusterapi
    read running total <<< $(kubectl get machines --context=kind-clusterapi \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /(r|R)unning/{count++} END{print count " " NR}') ;
    if [[ $total == "${TOTAL_MACHINE_COUNT}" && $running == "${TOTAL_MACHINE_COUNT}" ]]; then
      return 0
    fi
    read failed total <<< $(kubectl get machines --context=kind-clusterapi \
      -o json | jq -r '.items[].status.phase' | awk 'BEGIN{count=0} /(f|F)ailed/{count++} END{print count " " NR}') ;
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
  KUBECONFIG="${CAPG_WORKER_CLUSTER_KUBECONFIG}"
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
  NUM_NODES="$(kubectl get nodes --kubeconfig="$KUBECONFIG" \
    -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints}{"\n"}{end}' \
    | grep -cv "node-role.kubernetes.io/master" )"

  # wait for all the nodes to be ready
  kubectl wait --for=condition=Ready node --kubeconfig="$KUBECONFIG" --all || true

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

# generate manifests needed for creating the GCP cluster to run the tests
add_kustomize_patch() {
    # Enable the bits to inject a script that can pull newer versions of kubernetes
    if ! grep -i -wq "patchesStrategicMerge" "templates/kustomization.yaml"; then
        echo "patchesStrategicMerge:" >> "templates/kustomization.yaml"
    fi
    if ! grep -i -wq "kustomizeversions" "templates/kustomization.yaml"; then
        echo "- kustomizeversions.yaml" >> "templates/kustomization.yaml"
    fi
}

# setup kind, build kubernetes, create a cluster, run the e2es
main() {
  # skip the build image by default for CI
  # locally if want to build the image pass the flag --init-image
  SKIP_INIT_IMAGE="1"

  for arg in "$@"
  do
    if [[ "$arg" == "--verbose" ]]; then
      set -o xtrace
    fi
    if [[ "$arg" == "--clean" ]]; then
      cleanup
      return 0
    fi
    if [[ "$arg" == "--use-ci-artifacts" ]]; then
      USE_CI_ARTIFACTS="1"
      # when running the conformance that uses CI artifacts we need to build the node imade
      unset SKIP_INIT_IMAGE
    fi
    if [[ "$arg" == "--init-image" ]]; then
      unset SKIP_INIT_IMAGE
    fi
  done

  if [[ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
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

  # create temp dir and setup cleanup
  TMP_DIR=$(mktemp -d)
  SKIP_CLEANUP=${SKIP_CLEANUP:-""}
  if [[ -z "${SKIP_CLEANUP}" ]]; then
    trap exit-handler EXIT
  fi
  # ensure artifacts exists when not in CI
  export ARTIFACTS
  mkdir -p "${ARTIFACTS}/logs"

  # Initialize the necessary network requirements
  if [[ -n "${SKIP_INIT_NETWORK:-}" ]]; then
    echo "Skipping network initialization..."
  else
    init_networks
  fi

  if [[ -n ${CI_VERSION:-} || -n ${USE_CI_ARTIFACTS:-} ]]; then
    CI_VERSION=${CI_VERSION:-$(curl -sSL https://dl.k8s.io/ci/latest.txt)}
    KUBERNETES_VERSION=${CI_VERSION}
    KUBERNETES_MAJOR_VERSION=$(echo "${KUBERNETES_VERSION}" | cut -d '.' -f1 - | sed 's/v//')
    KUBERNETES_MINOR_VERSION=$(echo "${KUBERNETES_VERSION}" | cut -d '.' -f2 -)
  fi

  if [[ -n "${SKIP_INIT_IMAGE:-}" ]]; then
    echo "Skipping GCP image initialization..."
  else
    init_image
  fi

  # Build the images
  if [[ -n "${SKIP_IMAGE_BUILD:-}" ]]; then
    echo "Skipping Container image building..."
  else
    (GCP_PROJECT=${GCP_PROJECT} PULL_POLICY=Never make modules docker-build)
  fi

  # create cluster
  if [[ -n "${SKIP_CREATE_CLUSTER:-}" ]]; then
    echo "Skipping cluster creation..."
  else
    if [[ -n ${CI_VERSION:-} ]]; then
      echo "Adding kustomize patch for ci version..."
      add_kustomize_patch
    fi
    create_cluster
  fi

  # build k8s binaries and run conformance tests
  if [[ -z "${SKIP_TESTS:-}" && -z "${SKIP_RUN_TESTS:-}" ]]; then
    build_k8s
    run_tests
  fi
}

main "$@"
