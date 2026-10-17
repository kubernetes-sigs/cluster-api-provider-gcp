# Tracing For Development

Tracing can be a very useful tool when developing CAPG, below is some advice on how to enable this

## Tracing in a development cluster

If you develop CAPG with Tilt, add `"enable_tracing": true` to your `tilt-settings.json` (see the 
[development guide](./development.md#tilt-for-dev-in-capg)). Tilt then deploys an OpenTelemetry Collector 
from `hack/observability/otel-collector.yaml` into the `observability` namespace, enables tracing on the manager and 
points it at that collector.

The collector prints a summary of every span it receives to its log, and writes the spans as JSON lines to a file. 
A `log-collector` sidecar prints that file, so you can look at either:

```bash
kubectl -n observability logs deploy/otel-collector -c collector      # summary of each span
kubectl -n observability logs deploy/otel-collector -c log-collector  # every span as JSON
```

## Tracing in the end-to-end tests

The end-to-end tests deploy the same collector into the management cluster before the providers are installed, and 
enable tracing on CAPG. The first spec checks that spans for the cluster it creates reach the collector. Set 
`CAPG_ENABLE_TRACING=false` to run the tests without tracing; the conformance jobs do this, so the disabled code path 
is exercised as well.

The collector's logs are saved with the other controller logs in the test artifacts, in 
`clusters/bootstrap/logs/observability/otel-collector/<pod>/`. `collector.log` holds the summary of each span and 
`log-collector.log` holds every span as JSON lines.

## Viewing a captured run

The JSON lines in `log-collector.log` are in the OTLP JSON format, so they can be replayed through any OpenTelemetry 
Collector that has the `otlp_json_file` receiver (it is part of the `contrib` distribution). Save the file as 
`traces.jsonl` and use this configuration as `replay.yaml`:

```yaml
receivers:
  otlp_json_file:
    include: ["/in/traces.jsonl"]
    start_at: beginning
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    traces:
      receivers: [otlp_json_file]
      exporters: [debug]
```

Then run the collector, stopping it with Ctrl-C once it has printed the spans:

```bash
docker run --rm \
  -v "$PWD/traces.jsonl:/in/traces.jsonl:ro" \
  -v "$PWD/replay.yaml:/conf/replay.yaml:ro" \
  ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:0.162.0 \
  --config=/conf/replay.yaml
```

To look at the traces in a UI instead, you can use the script located [here](../../../../hack/view-traces.sh). The 
script will spin up Jaegar locally if pointed at a trace file.