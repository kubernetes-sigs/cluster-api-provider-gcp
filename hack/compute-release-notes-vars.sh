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

# This script computes and exports environment variables needed to run the
# release-notes tool. It is meant to be sourced, not executed directly.
# Required input: VERSION and GITHUB_TOKEN must be set in the caller's env.
# If you edit this script, run hack/test-compute-release-notes-vars.sh to
# verify it still works correctly.

echo "Computing release-notes env vars..."
echo ""

# Validate required input variables.
if [ -z "${VERSION:-}" ]; then echo "ERROR: VERSION is not set" >&2; return 1; fi
if [ -z "${GITHUB_TOKEN:-}" ]; then echo "ERROR: GITHUB_TOKEN is not set" >&2; return 1; fi

# Clear any stale values from a previous run.
unset RELEASE_BRANCH PREVIOUS_TAG

# Derive the release branch from VERSION (e.g. v1.12.1 -> release-1.12).
# Fall back to main if the branch doesn't exist yet (common for early betas).
RELEASE_BRANCH="release-$(echo "${VERSION}" | sed -E 's/^v([0-9]+\.[0-9]+)\..*/\1/')"
if ! git ls-remote --exit-code --heads upstream "${RELEASE_BRANCH}" > /dev/null 2>&1; then
  RELEASE_BRANCH="main"
fi

# Fetch the release branch and all tags from upstream.
git fetch upstream "${RELEASE_BRANCH}" --tags > /dev/null 2>&1

# Find the most recent stable release tag before VERSION using git ancestry.
# Lists all tags merged into VERSION (i.e. ancestors), then strips any
# prerelease suffix (e.g. v1.11.0-beta.0 -> v1.11.0) to resolve to the
# stable release for that series. Skips tags that resolve to VERSION itself.
#
# Note: the release-notes tool does not use git ancestry to determine which
# commits to include. Instead, it queries the GitHub API with Since/Until
# parameters based on the commit dates of PREVIOUS_TAG and VERSION. This is
# equivalent to:
#   git rev-list HEAD \
#     --after="$(git log -1 --format='%cI' "${PREVIOUS_TAG}")" \
#     --before="$(git log -1 --format='%cI' "${VERSION}")"
# So PREVIOUS_TAG determines the start of the date window, making it important
# that it points to the right release even if it's not a direct git ancestor.
# Examples:
#   v1.12.0       -> v1.11.0
#   v1.11.2       -> v1.11.1
#   v1.11.1       -> v1.11.0
#   v1.11.0       -> v1.10.0
#   v1.11.0-beta.0 -> v1.10.0
PREVIOUS_TAG=$(git tag --merged "${VERSION}" --sort=-v:refname | while read -r tag; do
  if [ "${tag}" = "${VERSION}" ]; then continue; fi
  resolved=$(echo "${tag}" | sed -E 's/-.*//')
  if [ "${resolved}" = "${VERSION}" ]; then continue; fi
  echo "${resolved}"; break
done)

export RELEASE_BRANCH PREVIOUS_TAG

echo "VERSION=${VERSION}"
echo "RELEASE_BRANCH=${RELEASE_BRANCH}"
echo "PREVIOUS_TAG=${PREVIOUS_TAG}"
echo ""
echo "These variables have been exported in your current shell."
echo "If the values look correct, proceed to run the release-notes tool."
