package exp

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api-provider-gcp/api/exp"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	exputil "sigs.k8s.io/cluster-api/exp/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// GCPMachineReconciler reconciles a GCPMachine object.
type GKEMachinePoolReconciler struct {
	client.Client
	Log              logr.Logger
	ReconcileTimeout time.Duration
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *GKEMachinePoolReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	logger := r.Log.WithValues("namespace", req.Namespace, "gkeMachine", req.Name)

	// Fetch the GCPMachine instance.
	gkeMachinePool := &exp.GKEMachinePool{}
	err := r.Get(ctx, req.NamespacedName, gkeMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Fetch the Machine Pool.
	machinePool, err := exputil.GetOwnerMachinePool(ctx, r.Client, gkeMachinePool.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machinePool == nil {
		logger.Info("Machine Pool Controller has not yet set OwnerRef")

		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		logger.Info("MachinePool is missing cluster label or cluster does not exist")

		return ctrl.Result{}, nil
	}

	if isPaused(cluster, gkeMachinePool) {
		logger.Info("GKEMachinePool or linked Cluster is marked as paused. Won't reconcile")

		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	gkeCluster := &exp.GKECluster{}

	gkeClusterName := client.ObjectKey{
		Namespace: gkeMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, gkeClusterName, gkeCluster); err != nil {
		logger.Info("GKECluster is not available yet")

		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("gkeCluster", gkeClusterName.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewGKEClusterScope(scope.GKEClusterScopeParams{
		Client:     r.Client,
		Logger:     logger,
		Cluster:    cluster,
		GKECluster: gkeCluster,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machinepool scope
	gkeMachinePoolScope, err := scope.NewGKEMachinePoolScope(scope.GKEMachinePoolScopeParams{
		Logger:     logger,
		Client:     r.Client,
		Cluster:    cluster,
		MachinePool:    machinePool,
		GKECluster: gkeCluster,
		GKEMachinePool: gkeMachinePool,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := gkeMachinePoolScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !gkeMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(gkeMachinePoolScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcile(ctx, gkeMachinePoolScope, clusterScope)
}

func (r *GKEMachinePoolReconciler) reconcileDelete(gkeMachinePool *scope.GKEMachinePoolScope, gkeClusterScope *scope.GKEClusterScope) (_ ctrl.Result, reterr error) {
	gkeMachinePool.Info("Handling deleted GKEMachinePool")

	// TODO(psu): Implement

	return ctrl.Result{}, nil
}

func (r *GKEMachinePoolReconciler) reconcile(_ context.Context, machineScope *scope.GKEMachinePoolScope, clusterScope *scope.GKEClusterScope) (ctrl.Result, error) {
	machineScope.Info("Reconciling GKEMachinePool")

	// TODO(psu): Implement

	return ctrl.Result{}, nil
}


func (r *GKEMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&exp.GKEMachinePool{}).
		Watches(
			&source.Kind{Type: &clusterv1exp.MachinePool{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("GKEManagedMachinePool")),
			},
		).Watches(
			&source.Kind{Type: &exp.GKECluster{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.GKEClusterToGKEMachinePools)},
		).WithEventFilter(pausePredicates).Build(r)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueGKEMachinePoolsForUnpausedCluster),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)

				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
			CreateFunc: func(e event.CreateEvent) bool {
				cluster := e.Object.(*clusterv1.Cluster)

				return !cluster.Spec.Paused
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		},
	)
}

func (r *GKEMachinePoolReconciler) requeueGKEMachinePoolsForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o.Object), "failed to get GKEMachinePools for unpaused Cluster")

		return nil
	}

	// Don't handle deleted clusters
	if !c.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	return r.requestsForCluster(c.Namespace, c.Name)
}

// GKEClusterToGKEMachinePools is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of GKEMachinePools.
func (r *GKEMachinePoolReconciler) GKEClusterToGKEMachinePools(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*infrav1.GCPCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a GCPCluster but got a %T", o.Object), "failed to get GCPMachine for GCPCluster")

		return nil
	}
	log := r.Log.WithValues("GCPCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return nil
	case err != nil:
		log.Error(err, "failed to get owning cluster")

		return nil
	}

	return r.requestsForCluster(cluster.Namespace, cluster.Name)
}


func (r *GKEMachinePoolReconciler) requestsForCluster(namespace, name string) []ctrl.Request {
	log := r.Log.WithValues("Cluster", name, "Namespace", namespace)
	labels := map[string]string{clusterv1.ClusterLabelName: name}
	machinePoolList := &exp.GKEMachinePool{}
	if err := r.Client.List(context.TODO(), machinePoolList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to get owned Machine Pools")

		return nil
	}

	result := make([]ctrl.Request, 0, 1)
	result = append(result, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: machinePoolList.Namespace, Name: machinePoolList.Name}})

	return result
}
