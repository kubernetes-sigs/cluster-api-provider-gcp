#!/usr/bin/env bash

# Copyright The Kubernetes Authors.
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

# Test for hack/compute-release-notes-vars.sh
# Verifies PREVIOUS_TAG resolution against the real git history.
# Requires the upstream remote to be configured.

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
PASS=0
FAIL=0

assert_previous_tag() {
  local version="${1}"
  local expected="${2}"

  # shellcheck source=hack/compute-release-notes-vars.sh
  VERSION="${version}" GITHUB_TOKEN="test" source "${SCRIPT_DIR}/compute-release-notes-vars.sh" > /dev/null 2>&1

  if [ "${PREVIOUS_TAG}" = "${expected}" ]; then
    echo "PASS: VERSION=${version} -> PREVIOUS_TAG=${PREVIOUS_TAG}"
    PASS=$((PASS + 1))
  else
    echo "FAIL: VERSION=${version} -> PREVIOUS_TAG=${PREVIOUS_TAG} (expected ${expected})"
    FAIL=$((FAIL + 1))
  fi
}

echo "Running compute-release-notes-vars tests..."
echo ""

assert_previous_tag "v1.12.0"        "v1.11.0"
assert_previous_tag "v1.11.2"        "v1.11.1"
assert_previous_tag "v1.11.1"        "v1.11.0"
assert_previous_tag "v1.11.0"        "v1.10.0"
assert_previous_tag "v1.11.0-beta.0" "v1.10.0"

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"

if [ "${FAIL}" -gt 0 ]; then
  return 1 2>/dev/null || exit 1
fi
