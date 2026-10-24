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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	ctrl "sigs.k8s.io/controller-runtime"
)

// StartSpan takes a given request and kind and creates the appropriate OTEL span for it, giving back a closure
// so it's easy to close the span once the reconciliation is complete
//
//nolint:spancheck // span.End is always called inside the closure but the linter can't see that
func StartSpan(ctx context.Context, kind string, req ctrl.Request) (context.Context, func(*error)) {
	tracer := otel.Tracer("sigs.k8s.io/cluster-api-provider-gcp")
	ctx, span := tracer.Start(ctx, kind+"/Reconcile", trace.WithAttributes(semconv.K8SNamespaceNameKey.String(req.Namespace), attribute.String("capg.object.name", req.Name)))
	return ctx, func(e *error) {
		if e != nil && *e != nil {
			span.RecordError(*e)
			span.SetStatus(codes.Error, (*e).Error())
		}
		span.End()
	}
}
