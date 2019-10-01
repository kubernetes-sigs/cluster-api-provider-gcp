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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName = "gcpcluster-controller"
)

// GCPClusterReconciler reconciles a GCPCluster object
type GCPClusterReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *GCPClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.GCPCluster{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *GCPClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	log := r.Log.WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("gcpCluster=%s", req.Name))

	// Fetch the GCPCluster instance
	gcpCluster := &infrav1.GCPCluster{}
	err := r.Get(ctx, req.NamespacedName, gcpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithName(gcpCluster.APIVersion)

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, gcpCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	// Create the scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     r.Client,
		Logger:     log,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
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

func (r *GCPClusterReconciler) reconcile(clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling GCPCluster")

	gcpCluster := clusterScope.GCPCluster

	// If the GCPCluster doesn't have our finalizer, add it.
	if !util.Contains(gcpCluster.Finalizers, infrav1.ClusterFinalizer) {
		gcpCluster.Finalizers = append(gcpCluster.Finalizers, infrav1.ClusterFinalizer)
	}

	computeSvc := compute.NewService(clusterScope)

	if err := computeSvc.ReconcileNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile network for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileFirewalls(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile firewalls for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileInstanceGroups(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile instance groups for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.ReconcileLoadbalancers(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile load balancers for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if gcpCluster.Status.Network.APIServerAddress == nil {
		clusterScope.Info("Waiting on API server Global IP Address")
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	gcpCluster.Status.APIEndpoints = []infrav1.APIEndpoint{
		{
			Host: *gcpCluster.Status.Network.APIServerAddress,
			Port: 443,
		},
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	gcpCluster.Status.Ready = true
	return reconcile.Result{}, nil
}

func (r *GCPClusterReconciler) reconcileDelete(clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling GCPCluster delete")

	computeSvc := compute.NewService(clusterScope)
	gcpCluster := clusterScope.GCPCluster

	if err := computeSvc.DeleteLoadbalancers(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting load balancer for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteInstanceGroups(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting instance groups for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteFirewalls(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting firewall rules for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	if err := computeSvc.DeleteNetwork(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting network for GCPCluster %s/%s", gcpCluster.Namespace, gcpCluster.Name)
	}

	// Cluster is deleted so remove the finalizer.
	clusterScope.GCPCluster.Finalizers = util.Filter(clusterScope.GCPCluster.Finalizers, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}
