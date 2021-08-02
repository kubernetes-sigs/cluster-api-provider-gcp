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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	//"google.golang.org/api/container/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	//clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	//"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/container"
	"time"

	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
)

// GKEClusterReconciler reconciles a GKECluster object
type GKEClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=infra.cluster.x-k8s.io,resources=gkeclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infra.cluster.x-k8s.io,resources=gkeclusters/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GKECluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *GKEClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(context.Background(), reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()
	log := r.Log.WithValues("namespace", req.Namespace, "gcpCluster", req.Name)

	// Fetch the GCPCluster instance
	gkeCluster := &infrav1alpha3.GKECluster{}
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

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	//if !gcpCluster.DeletionTimestamp.IsZero() {
	//	return r.reconcileDelete(clusterScope)
	//}

	// Handle non-deleted clusters
	return r.reconcile(clusterScope)
}

func (r *GKEClusterReconciler) reconcile(clusterScope *scope.GKEClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling GKECluster")

	gkeCluster := clusterScope.GKECluster

	// If the GKECluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(gkeCluster, infrav1alpha3.GKEClusterFinalizer)
	// Register the finalizer immediately to avoid orphaning AWS resources on delete
	if err := clusterScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	//containerSvc := container.NewService(clusterScope)

	//if err := computeSvc.ReconcileNetwork(); err != nil {
	//	return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile network for GCPCluster %s/%s", gkeCluster.Namespace, gkeCluster.Name)
	//}
	//
	//if err := computeSvc.ReconcileFirewalls(); err != nil {
	//	return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile firewalls for GCPCluster %s/%s", gkeCluster.Namespace, gkeCluster.Name)
	//}
	//
	//if err := computeSvc.ReconcileInstanceGroups(); err != nil {
	//	return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile instance groups for GCPCluster %s/%s", gkeCluster.Namespace, gkeCluster.Name)
	//}
	//
	//if err := computeSvc.ReconcileLoadbalancers(); err != nil {
	//	return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for GCPCluster %s/%s", gkeCluster.Namespace, gkeCluster.Name)
	//}
	//
	//if gkeCluster.Status.Network.APIServerAddress == nil {
	//	clusterScope.Info("Waiting on API server Global IP Address")
	//
	//	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	//}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	//gkeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
	//	Host: *gkeCluster.Status.Network.APIServerAddress,
	//	Port: 443,
	//}

	// Set FailureDomains on the GCPCluster Status
	//zones, err := computeSvc.GetZones()
	//if err != nil {
	//	return ctrl.Result{}, errors.Wrapf(err, "failed to get available zones for GCPCluster %s/%s", gkeCluster.Namespace, gkeCluster.Name)
	//}

	// FailureDomains list should be empty by default.
	//gkeCluster.Status.FailureDomains = make(clusterv1.FailureDomains, len(zones))

	// Iterate through all zones
	//for _, zone := range zones {
	//	If we have failuredomains in spec, see if this zone is in valid zone
	//	Add to the status _only_ if it's mentioned in the gkeCluster spec
	//	if len(gkeCluster.Spec.FailureDomains) > 0 {
	//		for _, fd := range gkeCluster.Spec.FailureDomains {
	//			if fd == zone {
	//				gkeCluster.Status.FailureDomains[zone] = clusterv1.FailureDomainSpec{
	//					ControlPlane: true,
	//				}
	//			}
	//		}
	//	} else {
	//		gkeCluster.Status.FailureDomains[zone] = clusterv1.FailureDomainSpec{
	//			ControlPlane: true,
	//		}
	//	}
	//}
	//
	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	gkeCluster.Status.Ready = true

	return ctrl.Result{}, nil
}


// SetupWithManager sets up the controller with the Manager.
func (r *GKEClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1alpha3.GKECluster{}).
		Complete(r)
}
