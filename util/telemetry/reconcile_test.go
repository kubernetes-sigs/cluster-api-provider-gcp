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
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestStartSpan(t *testing.T) {
	tests := []struct {
		name     string
		req      ctrl.Request
		wantErr  error
		wantCode codes.Code
	}{
		{
			name: "happy path",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "my-namespace",
					Name:      "my-cluster",
				},
			},
			wantErr:  nil,
			wantCode: codes.Unset,
		},
		{
			name: "error path",
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "my-namespace",
					Name:      "my-cluster",
				},
			},
			wantErr:  errors.New("reconciliation failed"),
			wantCode: codes.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// Save the old tracer provider and register cleanup
			oldTracerProvider := otel.GetTracerProvider()
			t.Cleanup(func() {
				otel.SetTracerProvider(oldTracerProvider)
			})

			spanRecorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
			otel.SetTracerProvider(provider)
			_, end := StartSpan(context.Background(), "myKind", tt.req)
			end(&tt.wantErr)
			recordedSpans := spanRecorder.Ended()

			g.Expect(recordedSpans).NotTo(BeEmpty(), "no spans were recorded")

			g.Expect(recordedSpans[0].Name()).To(Equal("myKind/Reconcile"))
			g.Expect(recordedSpans[0].Attributes()).To(ConsistOf(semconv.K8SNamespaceName("my-namespace"), attribute.String("capg.object.name", "my-cluster")))
			g.Expect(recordedSpans[0].Status().Code).To(Equal(tt.wantCode))
			if tt.wantErr != nil {
				g.Expect(recordedSpans[0].Events()).NotTo(BeEmpty(), "no events were recorded")
				g.Expect(recordedSpans[0].Events()[0].Name).To(Equal("exception"))
			} else {
				g.Expect(recordedSpans[0].Events()).To(BeEmpty())
			}
		})
	}
}
