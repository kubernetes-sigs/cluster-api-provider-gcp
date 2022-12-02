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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/container/nodepools"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/record"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GCPManagedMachinePoolReconciler reconciles a GCPManagedMachinePool object.
type GCPManagedMachinePoolReconciler struct {
	client.Client
	ReconcileTimeout time.Duration
	Scheme *runtime.Scheme
	WatchFilterValue string
}

// GetOwnerClusterKey returns only the Cluster name and namespace.
func GetOwnerClusterKey(obj metav1.ObjectMeta) (*client.ObjectKey, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return &client.ObjectKey{
				Namespace: obj.Namespace,
				Name:      ref.Name,
			}, nil
		}
	}
	return nil, nil
}

func machinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		m, ok := o.(*expclusterv1.MachinePool)
		if !ok {
			panic(fmt.Sprintf("Expected a MachinePool but got a %T", o))
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect
		infraGK := m.Spec.Template.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

func managedControlPlaneToManagedMachinePoolMapFunc(c client.Client, gvk schema.GroupVersionKind, log logr.Logger) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		ctx := context.Background()
		gcpManagedControlPlane, ok := o.(*infrav1exp.GCPManagedControlPlane)
		if !ok {
			panic(fmt.Sprintf("Expected a GCPManagedControlPlane but got a %T", o))
		}

		if !gcpManagedControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			return nil
		}

		clusterKey, err := GetOwnerClusterKey(gcpManagedControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "couldn't get GCPManagedControlPlane owner ObjectKey")
			return nil
		}
		if clusterKey == nil {
			return nil
		}

		managedPoolForClusterList := expclusterv1.MachinePoolList{}
		if err := c.List(
			ctx, &managedPoolForClusterList, client.InNamespace(clusterKey.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterKey.Name},
		); err != nil {
			log.Error(err, "couldn't list pools for cluster")
			return nil
		}

		mapFunc := machinePoolToInfrastructureMapFunc(gvk)

		var results []ctrl.Request
		for i := range managedPoolForClusterList.Items {
			managedPool := mapFunc(&managedPoolForClusterList.Items[i])
			results = append(results, managedPool...)
		}

		return results
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GCPManagedMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx).WithValues("controller", "GCPManagedMachinePool")

	gvk, err := apiutil.GVKForObject(new(infrav1exp.GCPManagedMachinePool), mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to find GVK for GCPManagedMachinePool")
	}

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.GCPManagedMachinePool{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(log, r.WatchFilterValue)).
		Watches(
			&source.Kind{Type: &expclusterv1.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(gvk)),
		).
		Watches(
			&source.Kind{Type: &infrav1exp.GCPManagedControlPlane{}},
			handler.EnqueueRequestsFromMapFunc(managedControlPlaneToManagedMachinePoolMapFunc(r.Client, gvk, log)),
		).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	clusterToObjectFunc, err := util.ClusterToObjectsMapper(r.Client, &infrav1exp.GCPManagedMachinePoolList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrap(err, "failed to create mapper for Cluster to GCPManagedMachinePools")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
		predicates.ClusterUnpausedAndInfrastructureReady(log),
	); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// getMachinePoolByName finds and return a Machine object using the specified params.
func getMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*expclusterv1.MachinePool, error) {
	m := &expclusterv1.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// getOwnerMachinePool returns the MachinePool object owning the current resource.
func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*expclusterv1.MachinePool, error) {
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

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedmachinepools/finalizers,verbs=update

func (r *GCPManagedMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx)

	// Get the managed machine pool
	gcpManagedMachinePool := &infrav1exp.GCPManagedMachinePool{}
	if err := r.Client.Get(ctx, req.NamespacedName, gcpManagedMachinePool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get the machine pool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, gcpManagedMachinePool.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner MachinePool from the API Server")
		return ctrl.Result{}, err
	}
	if machinePool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Get the cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Info("Failed to retrieve Cluster from MachinePool")
		return ctrl.Result{}, err
	}
	if annotations.IsPaused(cluster, gcpManagedMachinePool) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	gcpManagedControlPlaneKey := client.ObjectKey{
		Namespace: gcpManagedMachinePool.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	gcpManagedControlPlane := &infrav1exp.GCPManagedControlPlane{}
	if err := r.Client.Get(ctx, gcpManagedControlPlaneKey, gcpManagedControlPlane); err != nil {
		log.Info("Failed to retrieve ManagedControlPlane from ManagedMachinePool")
		return reconcile.Result{}, nil
	}

	if !gcpManagedControlPlane.Status.Ready {
		log.Info("Control plane is not ready yet")
		conditions.MarkFalse(gcpManagedMachinePool, infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.WaitingForGKEControlPlaneReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	managedMachinePoolScope, err := scope.NewManagedMachinePoolScope(scope.ManagedMachinePoolScopeParams{
		Client:     r.Client,
		Cluster:    cluster,
		GCPManagedControlPlane: gcpManagedControlPlane,
		GCPManagedMachinePool: gcpManagedMachinePool,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := managedMachinePoolScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machine pool
	if !gcpManagedMachinePool.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, managedMachinePoolScope)
	}

	// Handle non-deleted machine pool
	return r.reconcile(ctx, managedMachinePoolScope)
}

func (r *GCPManagedMachinePoolReconciler) reconcile(ctx context.Context, managedMachinePoolScope *scope.ManagedMachinePoolScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GCPManagedMachinePool")

	controllerutil.AddFinalizer(managedMachinePoolScope.GCPManagedMachinePool, infrav1exp.ManagedMachinePoolFinalizer)
	if err := managedMachinePoolScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	reconcilers := []cloud.Reconciler{
		nodepools.New(managedMachinePoolScope),
	}

	for _, r := range reconcilers {
		if err := r.Reconcile(ctx); err != nil {
			log.Error(err, "Reconcile error")
			record.Warnf(managedMachinePoolScope.GCPManagedMachinePool, "GCPManagedMachinePoolReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
	}

	if conditions.IsTrue(managedMachinePoolScope.GCPManagedMachinePool, infrav1exp.GKEMachinePoolCreatingCondition) {
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *GCPManagedMachinePoolReconciler) reconcileDelete(ctx context.Context, managedMachinePoolScope *scope.ManagedMachinePoolScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Deleting GCPManagedMachinePool")

	reconcilers := []cloud.Reconciler{
		nodepools.New(managedMachinePoolScope),
	}

	for _, r := range reconcilers {
		if err := r.Delete(ctx); err != nil {
			log.Error(err, "Reconcile error")
			record.Warnf(managedMachinePoolScope.GCPManagedMachinePool, "GCPManagedMachinePoolReconcile", "Reconcile error - %v", err)
			return ctrl.Result{}, err
		}
	}

	if conditions.Get(managedMachinePoolScope.GCPManagedMachinePool, infrav1exp.GKEMachinePoolDeletingCondition).Reason == infrav1exp.GKEMachinePoolDeletedReason {
		controllerutil.RemoveFinalizer(managedMachinePoolScope.GCPManagedMachinePool, infrav1exp.ManagedMachinePoolFinalizer)
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}
