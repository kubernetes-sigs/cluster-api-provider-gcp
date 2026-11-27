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

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# shellcheck source=hack/lib/versions.sh
source "${REPO_ROOT}/hack/lib/versions.sh"

if ! command -v gcloud >/dev/null ; then
  >&2 echo "'gcloud' CLI is not installed"
  exit 1
fi

if [ -z "$(gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null)" ]; then
  >&2 echo "'gcloud' CLI has no active account"
  exit 1
fi

load_ci_pins

if [ -n "${KUBERNETES_MINOR:-}" ]; then
  minor=$KUBERNETES_MINOR
fi

resolve_node_image

echo "KUBERNETES_VERSION=${KUBERNETES_VERSION}"
echo "IMAGE_ID=${IMAGE_ID}"

exit 0
