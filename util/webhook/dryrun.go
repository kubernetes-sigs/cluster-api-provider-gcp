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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api/util/topology"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type dryRunAware[T client.Object] struct {
	inner admission.Validator[T]
}

func (d dryRunAware[T]) ValidateCreate(ctx context.Context, t T) (warnings admission.Warnings, err error) {
	return d.inner.ValidateCreate(ctx, t)
}

func (d dryRunAware[T]) ValidateDelete(ctx context.Context, obj T) (warnings admission.Warnings, err error) {
	return d.inner.ValidateDelete(ctx, obj)
}

func (d dryRunAware[T]) ValidateUpdate(ctx context.Context, oldObj, newObj T) (warnings admission.Warnings, err error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("could not extract request from context: %v", err))
	}
	if topology.IsDryRunRequest(req, newObj) {
		// This might look odd but passing newObj twice means every immutability comparison will pass while the
		// rest of the validation remains valid.
		return d.inner.ValidateUpdate(ctx, newObj, newObj)
	}
	return d.inner.ValidateUpdate(ctx, oldObj, newObj)
}

// TopologyDryRunAware is a decorator to apply over the top of webhook invocations so we can
// follow CAPI conventions by skipping immutability checks during topology dry-runs.
func TopologyDryRunAware[T client.Object](inner admission.Validator[T]) admission.Validator[T] {
	return dryRunAware[T]{inner}
}
