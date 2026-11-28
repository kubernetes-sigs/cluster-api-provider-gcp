# Tracing

CAPG can export [OpenTelemetry](https://opentelemetry.io/) traces from its controller manager. Traces show how long each reconciliation takes and which GCP API calls it makes, which makes it much easier to investigate slow or failing reconciles. Tracing is disabled by default.

## What is traced

- Every reconcile of a CAPG resource produces a span named `<Kind>/Reconcile`, for example `GCPMachine/Reconcile`. The span carries the `k8s.namespace.name` and `capg.object.name` attributes, so you can find all of the reconciles for a single object. If the reconcile returns an error, it is recorded on the span and the span is marked as failed. The traced kinds are `GCPCluster`, `GCPMachine`, `GCPMachineTemplate`, `GCPMachinePool`, `GCPManagedCluster`, `GCPManagedControlPlane`, `GCPManagedMachinePool` and `GKEConfig`.
- The calls CAPG makes to GCP APIs during a reconcile appear as child spans of the reconcile. These are created by Google's client libraries, not by CAPG itself.

## Enabling tracing

When installing with `clusterctl`, set the `CAPG_ENABLE_TRACING` variable before initialising the provider:

```bash
export CAPG_ENABLE_TRACING=true
clusterctl init --infrastructure gcp
```

On a provider that is already installed, add the `--enable-tracing` flag to the arguments of the manager container in the `capg-controller-manager` Deployment (in the `capg-system` namespace):

```bash
kubectl -n capg-system patch deployment capg-controller-manager --type=json \
  -p '[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--enable-tracing"}]'
```

## Configuring the exporter

Traces are sent with the [OTLP](https://opentelemetry.io/docs/specs/otlp/) gRPC exporter. It is configured with the standard OpenTelemetry environment variables on the manager container:

| Variable | Default | Purpose |
|----------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `https://localhost:4317` | Address of the OTLP collector or backend. The scheme matters, see [Things to be aware of](#things-to-be-aware-of). |
| `OTEL_SERVICE_NAME` | `cluster-api-provider-gcp` | The service name reported with every span. |
| `OTEL_RESOURCE_ATTRIBUTES` | | Extra resource attributes, as comma-separated `key=value` pairs. |
| `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | SDK default | How traces are sampled. |

When installing with `clusterctl`, set the endpoint with the `CAPG_OTEL_EXPORTER_OTLP_ENDPOINT` variable. `clusterctl` copies it into `OTEL_EXPORTER_OTLP_ENDPOINT` on the manager container. For example, to send traces to a collector inside the cluster that does not use TLS:

```bash
export CAPG_ENABLE_TRACING=true
export CAPG_OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4317
clusterctl init --infrastructure gcp
```

On a provider that is already installed, set the environment variable on the Deployment directly (note the name without the `CAPG_` prefix):

```bash
kubectl -n capg-system set env deployment/capg-controller-manager \
  OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4317
```

See the OpenTelemetry documentation for the full list of [OTLP exporter](https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/) and [general SDK](https://opentelemetry.io/docs/languages/sdk-configuration/general/) settings, and the [Collector documentation](https://opentelemetry.io/docs/collector/) for setting up a backend to receive the traces.

## Things to be aware of

- Only OTLP over gRPC is supported. The `http/protobuf` protocol is not.
- The exporter connects with TLS unless the endpoint starts with `http://`. If your collector listens in plain text, use an `http://` endpoint, otherwise no traces will arrive and nothing is reported beyond a log message. The `OTEL_EXPORTER_OTLP_INSECURE` variable has no effect in the exporter CAPG uses, so setting it does not help.
- Spans are sent in batches. When the manager shuts down it flushes any pending spans, waiting for up to five seconds.
- If the backend cannot be reached, the failures are logged by the `telemetry` logger and the spans are dropped. The controllers keep running.
- You may see some red traces when viewing them in something like Jaeger. This is usually because of 404s, where controller tries to delete something that has already been deleted etc. Generally speaking there's nothing to worry about but if every trace is like this, you may have a problem!

To profile the controller instead, see the `--profiler-address` flag, which exposes the Go `pprof` endpoints.
