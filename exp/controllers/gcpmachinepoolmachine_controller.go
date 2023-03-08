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
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancegroupinstances"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPMachinePoolMachineReconciler reconciles a GCPMachinePoolMachine object and the corresponding MachinePool object.
type GCPMachinePoolMachineReconciler struct {
	client.Client
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepoolmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs;kubeadmconfigs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func (r *GCPMachinePoolMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx).WithValues("controller", "GCPMachinePoolMachine")

	gvk, err := apiutil.GVKForObject(new(infrav1exp.GCPMachinePoolMachine), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for GCPMachinePool")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.GCPMachinePoolMachine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&expclusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(gvk)),
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, gvk, mgr.GetClient(), &infrav1exp.GCPMachinePoolMachine{})),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// Reconcile handles GCPMachinePoolMachine events and reconciles the corresponding MachinePool.
func (r *GCPMachinePoolMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))

	defer cancel()

	log := ctrl.LoggerFrom(ctx)

	// Fetch the GCPMachinePoolMachine instance.
	gcpMachinePoolMachine := &infrav1exp.GCPMachinePoolMachine{}
	if err := r.Get(ctx, req.NamespacedName, gcpMachinePoolMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get the GCPMachinePool.
	gcpMachinePool, err := GetOwnerGCPMachinePool(ctx, r.Client, gcpMachinePoolMachine.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner GCPMachinePool from the API Server")
		return ctrl.Result{}, err
	}
	if gcpMachinePool == nil {
		log.Info("Waiting for GCPMachinePool Controller to set OwnerRef on GCPMachinePoolMachine")
		return ctrl.Result{}, nil
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
	if annotations.IsPaused(cluster, gcpMachinePoolMachine) {
		log.Info("GCPMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Create the logger with the GCPMachinePoolMachine name and delegate to the Reconcile method of the GCPMachinePoolMachineReconciler.
	log = log.WithValues("cluster", cluster.Name)
	gcpClusterName := client.ObjectKey{
		Namespace: gcpMachinePoolMachine.Namespace,
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
	machinePoolMachineScope, err := scope.NewMachinePoolMachineScope(scope.MachinePoolMachineScopeParams{
		Client:                r.Client,
		MachinePool:           machinePool,
		ClusterGetter:         clusterScope,
		GCPMachinePool:        gcpMachinePool,
		GCPMachinePoolMachine: gcpMachinePoolMachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create scope")
	}

	// Always close the scope when exiting this function so we can persist any GCPMachinePoolMachine changes.
	defer func() {
		if err := machinePoolMachineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machine pools
	if !gcpMachinePoolMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machinePoolMachineScope)
	}

	// Handle non-deleted machine pools
	return r.reconcileNormal(ctx, machinePoolMachineScope)
}

// reconcileNormal handles non-deleted GCPMachinePoolMachine instances.
func (r *GCPMachinePoolMachineReconciler) reconcileNormal(ctx context.Context, machinePoolMachineScope *scope.MachinePoolMachineScope) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling GCPMachinePoolMachine")

	// If the GCPMachinePoolMachine is in an error state, return early.
	if machinePoolMachineScope.GCPMachinePool.Status.FailureReason != nil || machinePoolMachineScope.GCPMachinePool.Status.FailureMessage != nil {
		log.Info("Error state detected, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	reconcilers := []cloud.ReconcilerWithResult{
		instancegroupinstances.New(machinePoolMachineScope),
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
			log.Error(err, "Failed to reconcile GCPMachinePoolMachine")
			record.Warnf(machinePoolMachineScope.GCPMachinePoolMachine, "FailedReconcile", "Failed to reconcile GCPMachinePoolMachine: %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue || res.RequeueAfter > 0 {
			return res, nil
		}
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles deleted GCPMachinePoolMachine instances.
func (r *GCPMachinePoolMachineReconciler) reconcileDelete(ctx context.Context, machinePoolMachineScope *scope.MachinePoolMachineScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GCPMachinePoolMachine delete")

	reconcilers := []cloud.ReconcilerWithResult{
		instancegroupinstances.New(machinePoolMachineScope),
	}

	for _, r := range reconcilers {
		res, err := r.Delete(ctx)
		if err != nil {
			log.Error(err, "Failed to reconcile GCPMachinePoolMachine delete")
			record.Warnf(machinePoolMachineScope.GCPMachinePoolMachine, "FailedDelete", "Failed to delete GCPMachinePoolMachine: %v", err)
			return ctrl.Result{}, err
		}
		if res.Requeue {
			return res, nil
		}
	}

	// Remove the finalizer from the GCPMachinePoolMachine.
	controllerutil.RemoveFinalizer(machinePoolMachineScope.GCPMachinePoolMachine, infrav1exp.GCPMachinePoolMachineFinalizer)

	return ctrl.Result{}, nil
}

// getGCPMachinePoolByName returns the GCPMachinePool object owning the current resource.
func getGCPMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*infrav1exp.GCPMachinePool, error) {
	gcpMachinePool := &infrav1exp.GCPMachinePool{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.Get(ctx, key, gcpMachinePool); err != nil {
		return nil, err
	}
	return gcpMachinePool, nil
}
