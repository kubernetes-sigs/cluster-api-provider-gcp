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

package webhook

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type fakeValidator[T client.Object] struct {
	Records map[string][]string
}

func (f fakeValidator[T]) ValidateCreate(_ context.Context, obj T) (warnings admission.Warnings, err error) {
	f.Records["create"] = []string{fmt.Sprintf("%s-%s", obj.GetNamespace(), obj.GetName())}
	return nil, nil
}

func (f fakeValidator[T]) ValidateUpdate(_ context.Context, oldObj, newObj T) (warnings admission.Warnings, err error) {
	f.Records["update"] = []string{
		fmt.Sprintf("%s-%s", oldObj.GetNamespace(), oldObj.GetName()),
		fmt.Sprintf("%s-%s", newObj.GetNamespace(), newObj.GetName()),
	}
	return nil, nil
}

func (f fakeValidator[T]) ValidateDelete(_ context.Context, obj T) (warnings admission.Warnings, err error) {
	f.Records["delete"] = []string{fmt.Sprintf("%s-%s", obj.GetNamespace(), obj.GetName())}
	return nil, nil
}

func TestTopologyDryRunAware(t *testing.T) {
	type testCase[T client.Object] struct {
		name               string
		dryRun             bool
		topologyAnnotation bool
		requestInContext   bool
		operation          func(ctx context.Context, decorated admission.Validator[T], oldCm, newCm T) (admission.Warnings, error)
		want               map[string][]string
		wantErr            bool
	}
	tests := []testCase[*corev1.ConfigMap]{
		{
			name:               "dry-run with topology annotation",
			dryRun:             true,
			topologyAnnotation: true,
			requestInContext:   true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], oldCm, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateUpdate(ctx, oldCm, newCm)
			},
			want: map[string][]string{
				"update": {"default-new", "default-new"},
			},
		},
		{
			name:             "dry-run without topology annotation",
			dryRun:           true,
			requestInContext: true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], oldCm, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateUpdate(ctx, oldCm, newCm)
			},
			want: map[string][]string{
				"update": {"default-old", "default-new"},
			},
		},
		{
			name:               "annotation without dry-run",
			topologyAnnotation: true,
			requestInContext:   true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], oldCm, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateUpdate(ctx, oldCm, newCm)
			},
			want: map[string][]string{
				"update": {"default-old", "default-new"},
			},
		},
		{
			name:             "no request in context returns bad request",
			requestInContext: false,
			wantErr:          true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], oldCm, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateUpdate(ctx, oldCm, newCm)
			},
			want: map[string][]string{},
		},
		{
			name:               "create passes through",
			dryRun:             true,
			topologyAnnotation: true,
			requestInContext:   true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], _, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateCreate(ctx, newCm)
			},
			want: map[string][]string{
				"create": {"default-new"},
			},
		},
		{
			name:               "delete passes through",
			dryRun:             true,
			topologyAnnotation: true,
			requestInContext:   true,
			operation: func(ctx context.Context, decorated admission.Validator[*corev1.ConfigMap], _, newCm *corev1.ConfigMap) (admission.Warnings, error) {
				return decorated.ValidateDelete(ctx, newCm)
			},
			want: map[string][]string{
				"delete": {"default-new"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fake := fakeValidator[*corev1.ConfigMap]{
				Records: map[string][]string{},
			}

			oldCm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "old",
					Namespace: "default",
				},
			}

			newCm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new",
					Namespace: "default",
				},
			}

			if tt.topologyAnnotation {
				newCm.Annotations = map[string]string{
					clusterv1.TopologyDryRunAnnotation: "true",
				}
			}

			// Prepare Context
			ctx := t.Context()
			if tt.requestInContext {
				ctx = admission.NewContextWithRequest(t.Context(), admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						DryRun: new(tt.dryRun),
					},
				})
			}

			// Run decorator
			_, err := tt.operation(ctx, TopologyDryRunAware(fake), &oldCm, &newCm)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.wantErr {
				g.Expect(apierrors.IsBadRequest(err)).To(BeTrue())
			}

			g.Expect(fake.Records).To(Equal(tt.want))
		})
	}
}
