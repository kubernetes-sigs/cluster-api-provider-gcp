#!/usr/bin/env bash

# Copyright 2025 The Kubernetes Authors.
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

YQ_VERSION="v4.45.4"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: go is not installed. Please install go first."
    exit 1
fi

# Install yq only if not already installed
if ! [ -x "$(command -v yq)" ]; then
    echo 'yq not found, installing'
    go install "github.com/mikefarah/yq/v4@${YQ_VERSION}"

    # Check if installation was successful
    if ! [ -x "$(command -v yq)" ]; then
        echo "error: yq installation failed."
        exit 1
    fi

    echo "yq ${YQ_VERSION} has been successfully installed."
fi
