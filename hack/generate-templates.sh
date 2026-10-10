#!/usr/bin/env bash

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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
TEMPLATES_DIR="$SCRIPT_DIR/../templates"
TEST_DIR="$SCRIPT_DIR/../test"
export CCM_TEMPLATE="$TEMPLATES_DIR/addons/gce-cloud-controller-manager.yaml"

# Check if the manifest's default major version for CCM matches `KUBERNETES_MINOR`
KUBERNETES_MINOR=$("$YQ" '.variables.KUBERNETES_MINOR' "$TEST_DIR/e2e/config/gcp-ci.yaml")
echo "Kubernetes Minor version is $KUBERNETES_MINOR"
REGEX="CCM_VERSION:=v([^.]*)"
CCM_VERSION_STRING=$("$YQ" '.spec.template.spec.containers[] | select(.name == "cloud-controller-manager").image' "$CCM_TEMPLATE")
if [[ $CCM_VERSION_STRING =~ $REGEX ]]
then
  DEFAULT_VALUE_MAJOR=${BASH_REMATCH[1]}
  echo "Default value for CCM major version is $DEFAULT_VALUE_MAJOR"
  if [ "1.$DEFAULT_VALUE_MAJOR" != "$KUBERNETES_MINOR" ]
  then
    echo "❌Default value in CCM_VERSION_STRING does not match KUBERNETES_MINOR in gcp-ci.yaml"
    exit 1
  else
    echo "✅ Kubernetes Minor matches CCM default major version "
  fi
else
  echo "❌Could not find CCM_VERSION_STRING in $CCM_TEMPLATE"
  exit 1
fi


for file in "$@"
do
  # shellcheck disable=SC2016
  if ! "$YQ" -e 'select(.kind=="ConfigMap" and (.metadata.name == "${CLUSTER_NAME}-crs-ccm"))' "$file" >/dev/null
  then
    echo "ERROR: Could not find CCM ConfigMap to replace" >&2
    exit 1
  fi
  # shellcheck disable=SC2016
  "$YQ" -e -i '(select(.kind=="ConfigMap" and (.metadata.name == "${CLUSTER_NAME}-crs-ccm")) | .data["cloud-controller-manager.yaml"]) =load_str(env(CCM_TEMPLATE))' "$file" > /dev/null
done
