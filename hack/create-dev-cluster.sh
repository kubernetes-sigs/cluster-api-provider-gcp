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

set -o errexit
set -o nounset
set -o pipefail

#Verify the required environment variables are present
: "${GCP_CREDENTIALS:?Environment variable empty or not defined.}"

make envsubst

export REGISTRY="${REGISTRY:-registry.local/fake}"

#cluster settings
export CLUSTER_NAME="${CLUSTER_NAME:-capg-test}"

#Google Cloud settings
export GCP_REGION="${GCP_REGION:-southcentralus}"
export GCP_PROJECT=${CLUSTER_NAME}

GCP_B64ENCODED_CREDENTIALS="$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"

export GCP_B64ENCODED_CREDENTIALS

#Machine settings
export CONTROL_PLANE_MACHINE_COUNT=${CONTROL_PLANE_MACHINE_COUNT:-1}
export GCP_CONTROL_PLANE_MACHINE_TYPE="${CONTROL_PLANE_MACHINE_TYPE:-n1-standard-2}"
export GCP_NODE_MACHINE_TYPE="${NODE_MACHINE_TYPE:-n1-standard-2}"
export WORKER_MACHINE_COUNT=${WORKER_MACHINE_COUNT:-1}
export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.20.9}"
export CLUSTER_TEMPLATE="${CLUSTER_TEMPLATE:-cluster-template.yaml}"

#identity settings
export GCP_NETWORK_NAME="default"

echo "================ DOCKER BUILD ==============="
PULL_POLICY=IfNotPresent make modules docker-build

echo "================ MAKE CLEAN ==============="
make clean

echo "================ KIND RESET ==============="
make kind-reset

echo "================ CREATE CLUSTER ==============="
make create-cluster
