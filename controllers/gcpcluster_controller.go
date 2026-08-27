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

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/firewalls"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/networks"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/subnets"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPClusterReconciler reconciles a GCPCluster object.
type GCPClusterReconciler struct {
	client.Client
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GCPClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx).WithValues("controller", "GCPCluster")

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.GCPCluster{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log, r.WatchFilterValue)).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetScheme(), log)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}

	clusterToInfraFn := util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("GCPCluster"), mgr.GetClient(), &infrav1.GCPCluster{})
	if err = c.Watch(
		source.Kind[client.Object](mgr.GetCache(), &clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(func(mapCtx context.Context, o client.Object) []reconcile.Request {
				requests := clusterToInfraFn(mapCtx, o)
				if requests == nil {
					return nil
				}

				gcpCluster := &infrav1.GCPCluster{}
				if err := r.Get(ctx, requests[0].NamespacedName, gcpCluster); err != nil {
					log.V(4).Error(err, "Failed to get GCP cluster")
					return nil
				}

				if annotations.IsExternallyManaged(gcpCluster) {
					log.V(4).Info("GCPCluster is externally managed, skipping mapping.")
					return nil
				}
				return requests
			}),
			predicates.ClusterUnpaused(mgr.GetScheme(), log),
		)); err != nil {
		return errors.Wrap(err, "failed adding a watch for ready clusters")
	}

	return nil
}

func (r *GCPClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := log.FromContext(ctx)
	gcpCluster := &infrav1.GCPCluster{}
	err := r.Get(ctx, req.NamespacedName, gcpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("GCPCluster resource not found or already deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Unable to fetch GCPCluster resource")
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
		log.Info("GCPCluster of linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:     r.Client,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := clusterScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !gcpCluster.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, clusterScope)
	}

	// Handle non-deleted clusters
	return r.reconcile(ctx, clusterScope)
}

func (r *GCPClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GCPCluster")

	controllerutil.AddFinalizer(clusterScope.GCPCluster, infrav1.ClusterFinalizer)
	if err := clusterScope.PatchObject(ctx); err != nil {
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

	failureDomains := make(clusterv1beta1.FailureDomains, len(zones))
	for _, zone := range zones {
		if len(clusterScope.GCPCluster.Spec.FailureDomains) > 0 {
			for _, fd := range clusterScope.GCPCluster.Spec.FailureDomains {
				if fd == zone.Name {
					failureDomains[zone.Name] = clusterv1beta1.FailureDomainSpec{
						ControlPlane: true,
					}
				}
			}
		} else {
			failureDomains[zone.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: true,
			}
		}
	}

	clusterScope.SetFailureDomains(failureDomains)

	if err := networks.New(clusterScope).Reconcile(ctx); err != nil {
		log.Error(err, "Error reconciling network resources")
		record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterNetworkReadyCondition, infrav1.NetworkReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterReadyCondition, infrav1.NetworkReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterNetworkReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.NetworkReconciliationFailedReason,
			Message: err.Error(),
		})
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.NetworkReconciliationFailedReason,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	v1beta1conditions.MarkTrue(clusterScope.ConditionSetter(), infrav1.GCPClusterNetworkReadyCondition)
	v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
		Type:   infrav1.GCPClusterNetworkReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.NetworkReadyReason,
	})

	if err := firewalls.New(clusterScope).Reconcile(ctx); err != nil {
		log.Error(err, "Error reconciling firewall resources")
		record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterFirewallRulesReadyCondition, infrav1.FirewallRulesReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterReadyCondition, infrav1.FirewallRulesReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterFirewallRulesReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FirewallRulesReconciliationFailedReason,
			Message: err.Error(),
		})
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FirewallRulesReconciliationFailedReason,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	v1beta1conditions.MarkTrue(clusterScope.ConditionSetter(), infrav1.GCPClusterFirewallRulesReadyCondition)
	v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
		Type:   infrav1.GCPClusterFirewallRulesReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.FirewallRulesReadyReason,
	})

	// Reconcile subnets before loadbalancers since subnet is needed for internal LB.
	if err := subnets.New(clusterScope).Reconcile(ctx); err != nil {
		log.Error(err, "Error reconciling subnet resources")
		record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterSubnetsReadyCondition, infrav1.SubnetsReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterReadyCondition, infrav1.SubnetsReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterSubnetsReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.SubnetsReconciliationFailedReason,
			Message: err.Error(),
		})
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.SubnetsReconciliationFailedReason,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	v1beta1conditions.MarkTrue(clusterScope.ConditionSetter(), infrav1.GCPClusterSubnetsReadyCondition)
	v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
		Type:   infrav1.GCPClusterSubnetsReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.SubnetsReadyReason,
	})

	if err := loadbalancers.New(clusterScope).Reconcile(ctx); err != nil {
		log.Error(err, "Error reconciling load balancer resources")
		record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterLoadBalancerReadyCondition, infrav1.LoadBalancerReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta1conditions.MarkFalse(clusterScope.ConditionSetter(), infrav1.GCPClusterReadyCondition, infrav1.LoadBalancerReconciliationFailedReason, clusterv1beta1.ConditionSeverityError, "%v", err)
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterLoadBalancerReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.LoadBalancerReconciliationFailedReason,
			Message: err.Error(),
		})
		v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
			Type:    infrav1.GCPClusterReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.LoadBalancerReconciliationFailedReason,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	v1beta1conditions.MarkTrue(clusterScope.ConditionSetter(), infrav1.GCPClusterLoadBalancerReadyCondition)
	v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
		Type:   infrav1.GCPClusterLoadBalancerReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.LoadBalancerReadyReason,
	})

	controlPlaneEndpoint := clusterScope.ControlPlaneEndpoint()
	if controlPlaneEndpoint.Host == "" {
		log.Info("GCPCluster does not have control-plane endpoint yet. Reconciling")
		record.Event(clusterScope.GCPCluster, "GCPClusterReconcile", "Waiting for control-plane endpoint")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	record.Eventf(clusterScope.GCPCluster, "GCPClusterReconcile", "Got control-plane endpoint - %s", controlPlaneEndpoint.Host)
	clusterScope.SetReady()
	v1beta1conditions.MarkTrue(clusterScope.ConditionSetter(), infrav1.GCPClusterReadyCondition)
	v1beta2conditions.Set(clusterScope.V1Beta2ConditionSetter(), metav1.Condition{
		Type:   infrav1.GCPClusterReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.ClusterReadyReason,
	})
	record.Event(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconciled")
	return ctrl.Result{}, nil
}

func (r *GCPClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling Delete GCPCluster")

	deleteReconcilers := []func(context.Context) error{
		loadbalancers.New(clusterScope).Delete,
		subnets.New(clusterScope).Delete,
		firewalls.New(clusterScope).Delete,
		networks.New(clusterScope).Delete,
	}

	for _, deleteFunc := range deleteReconcilers {
		if err := deleteFunc(ctx); err != nil {
			log.Error(err, "Reconcile error")
			record.Warnf(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconcile error - %v", err)
			return err
		}
	}

	controllerutil.RemoveFinalizer(clusterScope.GCPCluster, infrav1.ClusterFinalizer)
	record.Event(clusterScope.GCPCluster, "GCPClusterReconcile", "Reconciled")
	return nil
}
