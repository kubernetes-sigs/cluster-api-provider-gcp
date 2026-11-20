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

set -o errexit -o nounset -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
TOOLS_DIR="$SCRIPT_DIR/tools"

KUBE_CONTEXT="${KUBE_CONTEXT:-kind-capg}"
NIGHTLY="${NIGHTLY:-}"

if [ -z "${GCP_B64ENCODED_CREDENTIALS:-}" ]; then
  echo "GCP_B64ENCODED_CREDENTIALS must be set" >&2
  exit 1
fi

if ! command -v curl > /dev/null; then
  echo "curl must be installed" >&2
  exit 1
fi

if ! command -v jq > /dev/null; then
  echo "jq must be installed" >&2
  exit 1
fi

if ! "$TOOLS_DIR"/bin/kubectl --context "$KUBE_CONTEXT" get crd clusters.cluster.x-k8s.io >/dev/null; then
  echo "Cannot detect Cluster API installation in cluster" >&2
  exit 1
fi

if ! "$TOOLS_DIR"/bin/kubectl --context "$KUBE_CONTEXT" get crd certificates.cert-manager.io >/dev/null; then
  echo "Cannot detect cert-manager installation in cluster" >&2
  exit 1
fi

if [ -z "$NIGHTLY" ]; then
  listing_url="https://storage.googleapis.com/storage/v1/b/k8s-staging-cluster-api-gcp/o?prefix=components/nightly_main_&delimiter=/"
  if ! latest=$(curl -fsSL "$listing_url" | jq -er '.prefixes[-1]'); then
    echo "Could not find the latest nightly build" >&2
    exit 1
  fi
  NIGHTLY=${latest#components/}
  NIGHTLY=${NIGHTLY%/}
fi

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT
if ! curl -fsSL -o "$tmp/infrastructure-components.yaml" "https://storage.googleapis.com/k8s-staging-cluster-api-gcp/components/${NIGHTLY}/infrastructure-components.yaml"; then
  echo "Could not download nightly build: ${NIGHTLY}" >&2
  exit 1
fi

echo "Applying $NIGHTLY to context $KUBE_CONTEXT..."
"$TOOLS_DIR"/bin/envsubst < "${tmp}/infrastructure-components.yaml" | "$TOOLS_DIR"/bin/kubectl --context "${KUBE_CONTEXT}" apply --server-side --force-conflicts -f -

echo "Waiting for CAPG deployment to complete..."
"$TOOLS_DIR"/bin/kubectl --context "${KUBE_CONTEXT}" rollout status deployment/capg-controller-manager -n capg-system --timeout=120s
