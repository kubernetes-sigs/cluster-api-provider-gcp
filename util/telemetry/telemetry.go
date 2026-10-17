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

// Package telemetry exists to centralise functions that setup and maintain the telemetry
// for the controller
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"sigs.k8s.io/cluster-api-provider-gcp/version"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Setup sets up the resources, exporters etc. so telemetry can function correctly
func Setup(ctx context.Context) (func(context.Context) error, error) {
	serviceResource, err := newResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise telemetry resource: %w", err)
	}

	logger := ctrl.Log.WithName("telemetry")
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(err, "telemetry error")
	}))

	// Then the exporter
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise GRPC telemetry exporter: %w", err)
	}

	// Then the provider
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(serviceResource))
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}

func newResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceNameKey.String("cluster-api-provider-gcp"), semconv.ServiceVersion(version.Get().GitVersion)),
		resource.WithFromEnv())
}
