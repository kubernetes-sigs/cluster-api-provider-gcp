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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

// GetOwnerGCPMachinePool returns the GCPMachinePool object owning the current resource.
func GetOwnerGCPMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*infrav1exp.GCPMachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "GCPMachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == infrav1exp.GroupVersion.Group {
			return getGCPMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
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

// GCPMachinePoolMachineMapper returns a handler.ToRequestsFunc that watches for GCPMachinePool events and returns.
func GCPMachinePoolMachineMapper(scheme *runtime.Scheme, log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		gvk, err := apiutil.GVKForObject(new(infrav1exp.GCPMachinePool), scheme)
		if err != nil {
			log.Error(errors.WithStack(err), "failed to find GVK for GCPMachinePool")
			return nil
		}

		gcpMachinePoolMachine, ok := o.(*infrav1exp.GCPMachinePoolMachine)
		if !ok {
			log.Error(errors.Errorf("expected an GCPCluster, got %T instead", o), "failed to map GCPMachinePoolMachine")
			return nil
		}

		log := log.WithValues("GCPMachinePoolMachine", gcpMachinePoolMachine.Name, "Namespace", gcpMachinePoolMachine.Namespace)
		for _, ref := range gcpMachinePoolMachine.OwnerReferences {
			if ref.Kind != gvk.Kind {
				continue
			}

			gv, err := schema.ParseGroupVersion(ref.APIVersion)
			if err != nil {
				log.Error(errors.WithStack(err), "unable to parse group version", "APIVersion", ref.APIVersion)
				return nil
			}

			if gv.Group == gvk.Group {
				return []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      ref.Name,
							Namespace: gcpMachinePoolMachine.Namespace,
						},
					},
				}
			}
		}

		return nil
	}
}

// MachinePoolMachineHasStateOrVersionChange predicates any events based on changes to the GCPMachinePoolMachine status
// relevant for the GCPMachinePool controller.
func MachinePoolMachineHasStateOrVersionChange(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log := logger.WithValues("predicate", "MachinePoolModelHasChanged", "eventType", "update")

			oldGmp, ok := e.ObjectOld.(*infrav1exp.GCPMachinePoolMachine)
			if !ok {
				log.V(4).Info("Expected GCPMachinePoolMachine", "type", e.ObjectOld.GetObjectKind().GroupVersionKind().String())
				return false
			}
			log = log.WithValues("namespace", oldGmp.Namespace, "machinePoolMachine", oldGmp.Name)

			newGmp := e.ObjectNew.(*infrav1exp.GCPMachinePoolMachine)

			// if any of these are not equal, run the update
			shouldUpdate := oldGmp.Status.LatestModelApplied != newGmp.Status.LatestModelApplied ||
				oldGmp.Status.Version != newGmp.Status.Version ||
				oldGmp.Status.Ready != newGmp.Status.Ready

			if shouldUpdate {
				log.Info("machine pool machine predicate", "shouldUpdate", shouldUpdate)
			}
			return shouldUpdate
		},
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}
