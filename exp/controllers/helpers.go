/*
Copyright 2023 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

// GetOwnerMachinePool returns the MachinePool object owning the current resource.
func GetOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*expclusterv1.MachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "MachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == expclusterv1.GroupVersion.Group {
			return getMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// KubeadmConfigToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for KubeadmConfig events and returns.
func KubeadmConfigToInfrastructureMapFunc(_ context.Context, c client.Client, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		kc, ok := o.(*kubeadmv1.KubeadmConfig)
		if !ok {
			log.V(4).Info("attempt to map incorrect type", "type", fmt.Sprintf("%T", o))
			return nil
		}

		mpKey := client.ObjectKey{
			Namespace: kc.Namespace,
			Name:      kc.Name,
		}

		// fetch MachinePool to get reference
		mp := &expclusterv1.MachinePool{}
		if err := c.Get(ctx, mpKey, mp); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "failed to fetch MachinePool for KubeadmConfig")
			}
			return []reconcile.Request{}
		}

		ref := mp.Spec.Template.Spec.Bootstrap.ConfigRef
		if ref == nil {
			log.V(4).Info("fetched MachinePool has no Bootstrap.ConfigRef")
			return []reconcile.Request{}
		}
		sameKind := ref.Kind != o.GetObjectKind().GroupVersionKind().Kind
		sameName := ref.Name == kc.Name
		sameNamespace := ref.Namespace == kc.Namespace
		if !sameKind || !sameName || !sameNamespace {
			log.V(4).Info("Bootstrap.ConfigRef does not match",
				"sameKind", sameKind,
				"ref kind", ref.Kind,
				"other kind", o.GetObjectKind().GroupVersionKind().Kind,
				"sameName", sameName,
				"sameNamespace", sameNamespace,
			)
			return []reconcile.Request{}
		}

		key := client.ObjectKey{
			Namespace: kc.Namespace,
			Name:      kc.Name,
		}
		log.V(4).Info("adding KubeadmConfig to watch", "key", key)

		return []reconcile.Request{
			{
				NamespacedName: key,
			},
		}
	}
}
