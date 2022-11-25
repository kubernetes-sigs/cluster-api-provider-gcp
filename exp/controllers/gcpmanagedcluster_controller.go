/*
Copyright The Kubernetes Authors.

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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPManagedClusterReconciler reconciles a GCPManagedCluster object.
type GCPManagedClusterReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	WatchFilterValue string
	Scheme           *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GCPManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GCPManagedClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1exp.GCPManagedCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&source.Kind{Type: &infrav1exp.GCPManagedControlPlane{}},
			handler.EnqueueRequestsFromMapFunc(r.managedControlPlaneMapper(ctx)),
		).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating controller: %v", err)
	}

	if err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, infrav1exp.GroupVersion.WithKind("GCPManagedCluster"), mgr.GetClient(), &infrav1exp.GCPManagedCluster{})),
		predicates.ClusterUnpaused(log),
		predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue),
	); err != nil {
		return fmt.Errorf("adding watch for ready clusters: %v", err)
	}

	return nil
}

func (r *GCPManagedClusterReconciler) managedControlPlaneMapper(ctx context.Context) handler.MapFunc {
	return func(o client.Object) []ctrl.Request {
		log := ctrl.LoggerFrom(ctx)
		gcpManagedControlPlane, ok := o.(*infrav1exp.GCPManagedControlPlane)
		if !ok {
			log.Error(errors.Errorf("expected an GCPManagedControlPlane, got %T instead", o), "failed to map GCPManagedControlPlane")
			return nil
		}

		log = log.WithValues("objectMapper", "cpTomc", "gcpmanagedcontrolplane", klog.KRef(gcpManagedControlPlane.Namespace, gcpManagedControlPlane.Name))

		if !gcpManagedControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.Info("GCPManagedControlPlane has a deletion timestamp, skipping mapping")
			return nil
		}

		if gcpManagedControlPlane.Spec.Endpoint.IsZero() {
			log.V(2).Info("GCPManagedControlPlane has no  endpoint, skipping mapping")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, gcpManagedControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get owning cluster")
			return nil
		}
		if cluster == nil {
			log.Info("no owning cluster, skipping mapping")
			return nil
		}

		managedClusterRef := cluster.Spec.InfrastructureRef
		if managedClusterRef == nil || managedClusterRef.Kind != "GCPManagedCluster" {
			log.Info("InfrastructureRef is nil or not GCPManagedCluster, skipping mapping")
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      managedClusterRef.Name,
					Namespace: managedClusterRef.Namespace,
				},
			},
		}
	}
}
