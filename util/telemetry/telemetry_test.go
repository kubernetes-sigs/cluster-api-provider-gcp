/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

func Test_newResource(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		wantName string
	}{
		{
			name:     "check defaults apply",
			wantName: "cluster-api-provider-gcp",
		},
		{
			name: "check overrides",
			envVars: map[string]string{
				"OTEL_SERVICE_NAME": "foobar",
			},
			wantName: "foobar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			got, err := newResource(context.Background())
			require.NoError(t, err)
			assert.Contains(t, got.Attributes(), attribute.KeyValue{
				Key:   semconv.ServiceNameKey,
				Value: attribute.StringValue(tt.wantName),
			})
		})
	}
}

func TestSetup(t *testing.T) {
	oldTracerProvider := otel.GetTracerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTracerProvider)
	})

	// Point the exporter at an address nothing is listening on: Setup must still succeed because the
	// exporter connects lazily, and nothing is traced so there is nothing to flush on shutdown.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	shutdown, err := Setup(context.Background())
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	assert.IsType(t, &sdktrace.TracerProvider{}, otel.GetTracerProvider(), "Setup should register an SDK tracer provider")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, shutdown(shutdownCtx))
}

// TestGoogleAPIClientEmitsSpans guards the assumption this package relies on: the Google API client
// libraries trace their own HTTP calls, so CAPG needs no per-call instrumentation. If a dependency
// bump changes that, this test fails.
func TestGoogleAPIClientEmitsSpans(t *testing.T) {
	oldTracerProvider := otel.GetTracerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTracerProvider)
	})

	spanRecorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	// The client must be built after the provider is registered, and without option.WithHTTPClient,
	// which would bypass the library's own transport (and its tracing).
	service, err := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	_, err = service.Instances.Get("my-project", "us-central1-c", "my-instance").Context(context.Background()).Do()
	require.NoError(t, err)

	recordedSpans := spanRecorder.Ended()
	require.NotEmpty(t, recordedSpans, "the Google API client recorded no spans")

	clientSpans := 0
	for _, span := range recordedSpans {
		if span.SpanKind() == trace.SpanKindClient {
			clientSpans++
		}
	}
	assert.Positive(t, clientSpans, "expected at least one client span for the outgoing HTTP call")
}
