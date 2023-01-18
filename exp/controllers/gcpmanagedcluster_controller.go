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
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/networks"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/subnets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPManagedClusterReconciler reconciles a GCPManagedCluster object.
type GCPManagedClusterReconciler struct {
	client.Client
	WatchFilterValue string
	Scheme           *runtime.Scheme
	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedcontrolplanes,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GCPManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := log.FromContext(ctx)

	gcpCluster := &infrav1exp.GCPManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, gcpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("GCPManagedCluster resource not found or already deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Unable to fetch GCPManagedCluster resource")
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, gcpCluster.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to get owner cluster")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, gcpCluster) {
		log.Info("GCPManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	controlPlane := &infrav1exp.GCPManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
	}

	log.V(4).Info("getting control plane %s", controlPlaneRef)
	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		if !apierrors.IsNotFound(err) || gcpCluster.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, fmt.Errorf("failed to get control plane ref: %w", err)
		}
		controlPlane = nil
	}

	clusterScope, err := scope.NewManagedClusterScope(ctx, scope.ManagedClusterScopeParams{
		Client:                 r.Client,
		Cluster:                cluster,
		GCPManagedCluster:      gcpCluster,
		GCPManagedControlPlane: controlPlane,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !gcpCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcile(ctx, clusterScope)
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

func (r *GCPManagedClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.ManagedClusterScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GCPManagedCluster")

	controllerutil.AddFinalizer(clusterScope.GCPManagedCluster, infrav1exp.ClusterFinalizer)
	if err := clusterScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	region, err := clusterScope.Cloud().Regions().Get(ctx, meta.GlobalKey(clusterScope.Region()))
	if err != nil {
		return ctrl.Result{}, err
	}

	zones, err := clusterScope.Cloud().Zones().List(ctx, filter.Regexp("region", region.SelfLink))
	if err != nil {
		return ctrl.Result{}, err
	}

	failureDomains := make(clusterv1.FailureDomains, len(zones))
	for _, zone := range zones {
		failureDomains[zone.Name] = clusterv1.FailureDomainSpec{
			ControlPlane: false,
		}
	}
	clusterScope.SetFailureDomains(failureDomains)

	reconcilers := []cloud.Reconciler{
		networks.New(clusterScope),
		subnets.New(clusterScope),
	}

	for _, r := range reconcilers {
		if err := r.Reconcile(ctx); err != nil {
			log.Error(err, "Reconcile error")
			record.Warnf(clusterScope.GCPManagedCluster, "GCPManagedClusterReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
	}

	clusterScope.SetReady()
	record.Event(clusterScope.GCPManagedCluster, "GCPManagedClusterReconcile", "Ready")

	controlPlaneEndpoint := clusterScope.GCPManagedControlPlane.Spec.Endpoint
	if controlPlaneEndpoint.IsZero() {
		log.Info("GCPManagedControlplane does not have endpoint yet. Reconciling")
		record.Event(clusterScope.GCPManagedCluster, "GCPManagedClusterReconcile", "Waiting for control-plane endpoint")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	clusterScope.SetControlPlaneEndpoint(controlPlaneEndpoint)
	record.Eventf(clusterScope.GCPManagedCluster, "GCPManagedClusterReconcile", "Got control-plane endpoint - %s", controlPlaneEndpoint.Host)

	return ctrl.Result{}, nil
}

func (r *GCPManagedClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ManagedClusterScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Delete GCPManagedCluster")

	if clusterScope.GCPManagedControlPlane != nil {
		log.Info("GCPManagedControlPlane not deleted yet, retry later")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	reconcilers := []cloud.Reconciler{
		subnets.New(clusterScope),
		networks.New(clusterScope),
	}

	for _, r := range reconcilers {
		if err := r.Delete(ctx); err != nil {
			log.Error(err, "Reconcile error")
			record.Warnf(clusterScope.GCPManagedCluster, "GCPManagedClusterReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(clusterScope.GCPManagedCluster, infrav1exp.ClusterFinalizer)
	record.Event(clusterScope.GCPManagedCluster, "GCPClusterReconcile", "Reconciled")

	return ctrl.Result{}, nil
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
			log.V(2).Info("GCPManagedControlPlane has no endpoint, skipping mapping")
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
