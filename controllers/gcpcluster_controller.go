/*
Copyright 2019 The Kubernetes Authors.

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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute"
)

// GCPClusterReconciler reconciles a GCPCluster object
type GCPClusterReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *GCPClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.GCPCluster{}).
		Watches(
			&source.Kind{Type: &infrav1.GCPMachine{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.GCPMachineToGCPCluster)},
		).
		WithEventFilter(pausePredicates).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueGCPClusterForUnpausedCluster),
		},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				cluster := e.Object.(*clusterv1.Cluster)
				return !cluster.Spec.Paused
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
		},
	)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *GCPClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	log := r.Log.WithValues("namespace", req.Namespace, "gcpCluster", req.Name)

	// Fetch the GCPCluster instance
	gcpCluster := &infrav1.GCPCluster{}
	err := r.Get(ctx, req.NamespacedName, gcpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, gcpCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused(cluster, gcpCluster) {
		log.Info("GCPCluster of linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     r.Client,
		Logger:     log,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
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
		return r.reconcileDelete(clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcile(clusterScope)
}

func (r *GCPClusterReconciler) reconcile(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling GCPCluster")

	gcpCluster := clusterScope.GCPCluster

	// If the GCPCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(gcpCluster, infrav1.ClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning AWS resources on delete
	if err := clusterScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	computeSvc := compute.NewService(clusterScope)

	if err := computeSvc.ReconcileNetwork(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile network for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileFirewalls(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile firewalls for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileInstanceGroups(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile instance groups for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileLoadbalancers(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if gcpCluster.Status.Network.APIServerAddress == nil {
		clusterScope.Info("Waiting on API server Global IP Address")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	gcpCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: *gcpCluster.Status.Network.APIServerAddress,
		Port: 443,
	}

	// Set FailureDomains on the GCPCluster Status
	zones, err := computeSvc.GetZones()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get available zones for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	// FailureDomains list should be empty by default.
	gcpCluster.Status.FailureDomains = make(clusterv1.FailureDomains, len(zones))

	// Iterate through all zones
	for _, zone := range zones {
		// If we have failuredomains in spec, see if this zone is in valid zone
		// Add to the status _only_ if it's mentioned in the gcpCluster spec
		if len(gcpCluster.Spec.FailureDomains) > 0 {
			for _, fd := range gcpCluster.Spec.FailureDomains {
				if fd == zone {
					gcpCluster.Status.FailureDomains[zone] = clusterv1.FailureDomainSpec{
						ControlPlane: true,
					}
				}
			}
		} else {
			gcpCluster.Status.FailureDomains[zone] = clusterv1.FailureDomainSpec{
				ControlPlane: true,
			}
		}
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	gcpCluster.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *GCPClusterReconciler) reconcileDelete(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling GCPCluster delete")

	computeSvc := compute.NewService(clusterScope)
	gcpCluster := clusterScope.GCPCluster

	if err := computeSvc.DeleteLoadbalancers(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting load balancer for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteInstanceGroups(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting instance groups for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteFirewalls(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting firewall rules for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteNetwork(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error deleting network for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(clusterScope.GCPCluster, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *GCPClusterReconciler) requeueGCPClusterForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o.Object), "failed to get GCPClusters for unpaused Cluster")
		return nil
	}

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	// Make sure the ref is set
	if c.Spec.InfrastructureRef == nil {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
		},
	}
}

// GCPMachineToGCPCluster is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of GCPCluster.
func (r *GCPClusterReconciler) GCPMachineToGCPCluster(o handler.MapObject) []ctrl.Request {
	m, ok := o.Object.(*infrav1.GCPMachine)
	if !ok {
		r.Log.Error(errors.Errorf("expected a GCPMachine but got a %T", o.Object), "failed to get GCPCluster for GCPMachine")
		return nil
	}
	log := r.Log.WithValues("GCPMachine", m.Name, "Namespace", m.Namespace)

	c, err := util.GetOwnerCluster(context.TODO(), r.Client, m.ObjectMeta)
	switch {
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return nil
	case apierrors.IsNotFound(err) || c == nil || c.Spec.InfrastructureRef == nil:
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.InfrastructureRef.Name},
		},
	}
}
