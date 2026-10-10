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

TRACES_FILE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OTEL_IMAGE=$("${SCRIPT_DIR}/tools/bin/yq" -e 'select(.kind == "Deployment") | .spec.template.spec.containers[] | select(.name == "collector") | .image' "${SCRIPT_DIR}/observability/otel-collector.yaml") || exit 1

docker network create trace-replay 2>/dev/null || true
docker rm -f jaeger-trace-replay 2>/dev/null || true
docker run -d --name jaeger-trace-replay --network trace-replay \
    -p 16686:16686 \
    jaegertracing/jaeger:latest

echo "Open Jaeger UI at http://localhost:16686"
echo "Run 'docker rm -f jaeger-trace-replay && docker network rm trace-replay' to clean up"

docker run --rm \
    --network trace-replay \
    -v "$(realpath "$TRACES_FILE")":/traces/traces.otlp.json:ro \
    -v "${SCRIPT_DIR}/observability/trace_replay.yaml":/etc/otelcol-contrib/config.yaml:ro \
    "${OTEL_IMAGE}"

