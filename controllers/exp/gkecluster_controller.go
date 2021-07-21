package exp

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	//infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	exp "sigs.k8s.io/cluster-api-provider-gcp/api/exp"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// GKEClusterReconciler reconciles a GKECluster object.
type GKEClusterReconciler struct {
	client.Client
	Log              logr.Logger
	ReconcileTimeout time.Duration
}

func (r *GKEClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&exp.GKECluster{}).
		Watches(
			&source.Kind{Type: &exp.GKEMachinePool{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.GKEMachinePoolToGKECluster)},
		).
		WithEventFilter(pausePredicates).Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueGKEClusterForUnpausedCluster),
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

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gkeclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gkeclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *GKEClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "gkeCluster", req.Name)

	// Fetch the GKECluster instance
	gkeCluster := &exp.GKECluster{}
	err := r.Get(ctx, req.NamespacedName, gkeCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, gkeCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused(cluster, gkeCluster) {
		log.Info("GKECluster of linked Cluster is marked as paused. Won't reconcile")

		return ctrl.Result{}, nil
	}

	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")

		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Create the scope.
	clusterScope, err := scope.NewGKEClusterScope(scope.GKEClusterScopeParams{
		Client:     r.Client,
		Logger:     log,
		Cluster:    cluster,
		GKECluster: gkeCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GKEMachine changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !gkeCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcile(clusterScope)
}


func (r *GKEClusterReconciler) requeueGKEClusterForUnpausedCluster(o handler.MapObject) []ctrl.Request {
	c, ok := o.Object.(*clusterv1.Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o.Object), "failed to get GKEClusters for unpaused Cluster")

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


// GKEMachineToGKECluster is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of GKECluster.
func (r *GKEClusterReconciler) GKEMachinePoolToGKECluster(o handler.MapObject) []ctrl.Request {
	m, ok := o.Object.(*exp.GKEMachinePool)
	if !ok {
		r.Log.Error(errors.Errorf("expected a GKEMachinePool but got a %T", o.Object), "failed to get GKECluster for GKEMachinePool")

		return nil
	}
	log := r.Log.WithValues("GKEMachine", m.Name, "Namespace", m.Namespace)

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

func (r *GKEClusterReconciler) reconcileDelete(clusterScope *scope.GKEClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling GKECluster delete")

	// TODO(psu): Implement

	return ctrl.Result{}, nil
}

func (r *GKEClusterReconciler) reconcile(clusterScope *scope.GKEClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling GKECluster")

	gkeCluster := clusterScope.GKECluster
	// TODO(psu): Implement

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	gkeCluster.Status.Ready = true

	return ctrl.Result{}, nil
}