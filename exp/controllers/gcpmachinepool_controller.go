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
	"time"

	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancegroups"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kubeadmv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPMachinePoolReconciler reconciles a GCPMachinePool object and the corresponding MachinePool object.
type GCPMachinePoolReconciler struct {
	client.Client
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs;kubeadmconfigs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func (r *GCPMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx).WithValues("controller", "GCPMachinePool")

	gvk, err := apiutil.GVKForObject(new(infrav1exp.GCPMachinePool), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for GCPMachinePool")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.GCPMachinePool{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&expclusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(gvk)),
		).
		// watch for changes in KubeadmConfig to sync bootstrap token
		Watches(
			&kubeadmv1.KubeadmConfig{},
			handler.EnqueueRequestsFromMapFunc(KubeadmConfigToInfrastructureMapFunc(ctx, r.Client, log)),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Watch for changes in the GCPMachinePool instances and enqueue the GCPMachinePool object for the controller
	if err := c.Watch(
		source.Kind(mgr.GetCache(), &infrav1exp.GCPMachinePoolMachine{}),
		handler.EnqueueRequestsFromMapFunc(GCPMachinePoolMachineMapper(mgr.GetScheme(), log)),
		MachinePoolMachineHasStateOrVersionChange(log),
		predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for GCPMachinePoolMachine")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, gvk, mgr.GetClient(), &infrav1exp.GCPMachinePool{})),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// Reconcile handles GCPMachinePool events and reconciles the corresponding MachinePool.
func (r *GCPMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))

	defer cancel()

	log := ctrl.LoggerFrom(ctx)

	// Fetch the GCPMachinePool instance.
	gcpMachinePool := &infrav1exp.GCPMachinePool{}
	if err := r.Get(ctx, req.NamespacedName, gcpMachinePool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get the MachinePool.
	machinePool, err := GetOwnerMachinePool(ctx, r.Client, gcpMachinePool.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner MachinePool from the API Server")
		return ctrl.Result{}, err
	}
	if machinePool == nil {
		log.Info("Waiting for MachinePool Controller to set OwnerRef on GCPMachinePool")
		return ctrl.Result{}, nil
	}

	// Get the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{}, err
	}
	if annotations.IsPaused(cluster, gcpMachinePool) {
		log.Info("GCPMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)
	gcpClusterName := client.ObjectKey{
		Namespace: gcpMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	gcpCluster := &infrav1.GCPCluster{}
	if err := r.Client.Get(ctx, gcpClusterName, gcpCluster); err != nil {
		log.Info("GCPCluster is not available yet")
		return ctrl.Result{}, err
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:     r.Client,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create scope")
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:         r.Client,
		MachinePool:    machinePool,
		GCPMachinePool: gcpMachinePool,
		ClusterGetter:  clusterScope,
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create scope")
	}

	// Make sure bootstrap data is available and populated.
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	defer func() {
		if err := machinePoolScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machine pools
	if !gcpMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePoolScope)
	}

	// Handle non-deleted machine pools
	return r.reconcileNormal(ctx, machinePoolScope)
}

// reconcileNormal handles non-deleted GCPMachinePools.
func (r *GCPMachinePoolReconciler) reconcileNormal(ctx context.Context, machinePoolScope *scope.MachinePoolScope) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling GCPMachinePool")

	// If the GCPMachinePool has a status failure reason, return early. This is to avoid attempting to do anything to the GCPMachinePool if there is a known problem.
	if machinePoolScope.GCPMachinePool.Status.FailureReason != nil || machinePoolScope.GCPMachinePool.Status.FailureMessage != nil {
		log.Info("Found failure reason or message, returning early")
		return ctrl.Result{}, nil
	}

	// If the GCPMachinePool doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.GCPMachinePool, expclusterv1.MachinePoolFinalizer)
	if err := machinePoolScope.PatchObject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	reconcilers := []cloud.ReconcilerWithResult{
		instancegroups.New(machinePoolScope),
	}

	for _, r := range reconcilers {
		res, err := r.Reconcile(ctx)
		if err != nil {
			var e *apierror.APIError
			if ok := errors.As(err, &e); ok {
				if e.GRPCStatus().Code() == codes.FailedPrecondition {
					log.Info("GCP API returned a failed precondition error, retrying")
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			}
			log.Error(err, "Failed to reconcile GCPMachinePool")
			record.Warnf(machinePoolScope.GCPMachinePool, "FailedReconcile", "Failed to reconcile GCPMachinePool: %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue {
			log.Info("Requeueing GCPMachinePool reconcile")
			return res, nil
		}
	}

	if machinePoolScope.NeedsRequeue() {
		log.Info("Requeueing GCPMachinePool reconcile", "RequeueAfter", 30*time.Second)
		return reconcile.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles deleted GCPMachinePools.
func (r *GCPMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.MachinePoolScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GCPMachinePool delete")

	reconcilers := []cloud.ReconcilerWithResult{
		instancegroups.New(machinePoolScope),
	}

	for _, r := range reconcilers {
		res, err := r.Delete(ctx)
		if err != nil {
			log.Error(err, "Failed to reconcile GCPMachinePool")
			record.Warnf(machinePoolScope.GCPMachinePool, "FailedReconcile", "Failed to reconcile GCPMachinePool: %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue {
			return res, nil
		}
	}

	// Remove the finalizer from the GCPMachinePool
	controllerutil.RemoveFinalizer(machinePoolScope.GCPMachinePool, expclusterv1.MachinePoolFinalizer)

	return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
}
