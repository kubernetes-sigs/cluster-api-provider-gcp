package otel

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/pkg/version"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func RegisterTracing(ctx context.Context, log logr.Logger) error {

	tracerProvider, err := SetUpTracing(ctx)
	if err != nil {
		return err
	}

	// Safely shut down the tracer provider when context terminates
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Error(err, "failed to shut down tracer provider")
		}
	}()

	return nil
}

func newExporter(ctx context.Context) (*otlptrace.Exporter, error) {

	conn, err := grpc.DialContext(ctx, "opentelemetry-collector:4317",
		// Using non-TLS connection for dev environment
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create gRPC connection to collector for opentelemetry")
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))

	if err != nil {
		return nil, errors.Wrap(err, "failed to create trace exporter for opentelemetry")
	}

	return traceExporter, nil
}

func SetUpTracing(ctx context.Context) (*trace.TracerProvider, error) {

	traceExporter, err := newExporter(ctx)

	if err != nil {
		return nil, err
	}

	// labels/tags/res common to all traces
	// TODO: consider to add more fields
	resource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("capg"),
			attribute.String("exporter", "otlpgrpc"),
			attribute.String("version", version.Get().String()),
		),
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create opentelemetry resource")
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(resource),
		// TODO: dynamic sampling rate?
		// sampling rate based on parent span = 60%
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.6))),
	)

	otel.SetTracerProvider(traceProvider)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
		),
	)

	return traceProvider, nil
}
