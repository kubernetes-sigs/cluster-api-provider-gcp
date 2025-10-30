/*
Copyright 2020 The Kubernetes Authors.

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

// Package controllers provides experimental API controllers.
package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancegroupmanagers"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancetemplates"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/capiutils"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GCPMachinePoolReconciler reconciles a GCPMachinePool object.
type GCPMachinePoolReconciler struct {
	Client           client.Client
	Recorder         record.EventRecorder
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile is the reconciliation loop for GCPMachinePool.
func (r *GCPMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx)

	// Fetch the GCPMachinePool .
	gcpMachinePool := &expinfrav1.GCPMachinePool{}
	err := r.Client.Get(ctx, req.NamespacedName, gcpMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the CAPI MachinePool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, gcpMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		log.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}
	log = log.WithValues("machinePool", klog.KObj(machinePool))

	// Fetch the Cluster.
	clusterObj, err := capiutils.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}
	log = log.WithValues("cluster", klog.KObj(clusterObj))

	if capiutils.IsPaused(clusterObj, gcpMachinePool) {
		log.Info("GCPMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	_, clusterScope, err := r.getInfraCluster(ctx, clusterObj, gcpMachinePool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting infra provider cluster or control plane object: %w", err)
	}
	if clusterScope == nil {
		log.Info("GCPCluster or GCPManagedControlPlane is not ready yet")
		return ctrl.Result{}, nil
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		ClusterGetter:  clusterScope,
		Client:         r.Client,
		MachinePool:    machinePool,
		GCPMachinePool: gcpMachinePool,
	})
	if err != nil {
		log.Error(err, "failed to create scope")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		// set Ready condition before GCPMachinePool is patched
		conditions.SetSummary(machinePoolScope.GCPMachinePool,
			conditions.WithConditions(
				expinfrav1.MIGReadyCondition,
				expinfrav1.InstanceTemplateReadyCondition,
			),
			conditions.WithStepCounterIfOnly(
				expinfrav1.MIGReadyCondition,
				expinfrav1.InstanceTemplateReadyCondition,
			),
		)

		if err := machinePoolScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !gcpMachinePool.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, machinePoolScope)
	}

	return r.reconcile(ctx, machinePoolScope)
}

func (r *GCPMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&expinfrav1.GCPMachinePool{}).
		Watches(
			&clusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(expinfrav1.GroupVersion.WithKind("GCPMachinePool"))),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), log.FromContext(ctx), r.WatchFilterValue)).
		WithEventFilter(
			predicate.Funcs{
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for GCPMachinePool resources only
				UpdateFunc: func(e event.UpdateEvent) bool {
					if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind != "GCPMachinePool" {
						return true
					}

					oldCluster := e.ObjectOld.(*expinfrav1.GCPMachinePool).DeepCopy()
					newCluster := e.ObjectNew.(*expinfrav1.GCPMachinePool).DeepCopy()

					oldCluster.Status = expinfrav1.GCPMachinePoolStatus{}
					newCluster.Status = expinfrav1.GCPMachinePoolStatus{}

					oldCluster.ObjectMeta.ResourceVersion = ""
					newCluster.ObjectMeta.ResourceVersion = ""

					return !cmp.Equal(oldCluster, newCluster)
				},
			},
		).
		Complete(r)
}

func (r *GCPMachinePoolReconciler) reconcile(ctx context.Context, machinePoolScope *scope.MachinePoolScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling GCPMachinePool")

	// If the GCPMachinepool doesn't have our finalizer, add it
	if controllerutil.AddFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer) {
		// Register finalizer immediately to avoid orphaning GCP resources
		if err := machinePoolScope.PatchObject(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	// // If the GCPMachine is in an error state, return early.
	// if machinePoolScope.HasFailed() {
	// 	log.Info("Error state detected, skipping reconciliation")

	// 	// FUTURE: If we are in a failed state, delete the secret regardless of instance state

	// 	return ctrl.Result{}, nil
	// }

	// if !machinePoolScope.Cluster.Status.InfrastructureReady {
	// 	log.Info("Cluster infrastructure is not ready yet")
	// 	conditions.MarkFalse(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
	// 	return ctrl.Result{}, nil
	// }

	// Make sure bootstrap data is available and populated
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	instanceTemplateKey, err := instancetemplates.New(machinePoolScope).Reconcile(ctx)
	if err != nil {
		log.Error(err, "Error reconciling instanceTemplate")
		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition, expinfrav1.InstanceTemplateNotFoundReason, "%s", err.Error())
		return ctrl.Result{}, err
	}

	// set the InstanceTemplateReadyCondition condition
	conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition)

	igm, err := instancegroupmanagers.New(machinePoolScope).Reconcile(ctx, instanceTemplateKey)
	if err != nil {
		log.Error(err, "Error reconciling instanceGroupManager")
		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
		return ctrl.Result{}, err
	}

	// set the MIGReadyCondition condition
	conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition)

	igmInstances, err := instancegroupmanagers.New(machinePoolScope).ListInstances(ctx, igm)
	if err != nil {
		log.Error(err, "Error listing instances in instanceGroupManager")
		return ctrl.Result{}, err
	}

	providerIDList := make([]string, len(igmInstances))

	for i, instance := range igmInstances {
		var providerID string

		// Convert instance URL to providerID format
		u := instance.Instance
		u = strings.TrimPrefix(u, "https://www.googleapis.com/compute/v1/")
		tokens := strings.Split(u, "/")
		if len(tokens) == 6 && tokens[0] == "projects" && tokens[2] == "zones" && tokens[4] == "instances" {
			providerID = fmt.Sprintf("gce://%s/%s/%s", tokens[1], tokens[3], tokens[5])
		} else {
			return ctrl.Result{}, fmt.Errorf("unexpected instance URL format: %s", instance.Instance)
		}

		providerIDList[i] = providerID
	}

	// FUTURE: do we need to verify that the instances are actually running?
	machinePoolScope.GCPMachinePool.Spec.ProviderIDList = providerIDList
	machinePoolScope.GCPMachinePool.Status.Replicas = int32(len(providerIDList))
	machinePoolScope.GCPMachinePool.Status.Ready = true

	// Requeue so that we can keep the spec.providerIDList and status in sync with the MIG.
	// This is important for scaling up and down, as the CAPI MachinePool controller relies on
	// the providerIDList to determine which machines belong to the MachinePool.
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *GCPMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.MachinePoolScope) error {
	log := log.FromContext(ctx)

	log.Info("Handling deleted GCPMachinePool")

	if err := instancegroupmanagers.New(machinePoolScope).Delete(ctx); err != nil {
		log.Error(err, "Error deleting instanceGroupManager")
		r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instancegroupmanager: %v", err)

		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
		return err
	}

	if err := instancetemplates.New(machinePoolScope).Delete(ctx); err != nil {
		log.Error(err, "Error deleting instanceTemplates")
		r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instance template: %v", err)

		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition, expinfrav1.InstanceTemplateReconcileFailedReason, "%s", err.Error())
		return err
	}

	// remove finalizer
	controllerutil.RemoveFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer)

	return nil
}

func (r *GCPMachinePoolReconciler) getInfraCluster(ctx context.Context, cluster *clusterv1.Cluster, gcpMachinePool *expinfrav1.GCPMachinePool) (*infrav1.GCPCluster, *scope.ClusterScope, error) {
	gcpCluster := &infrav1.GCPCluster{}

	gcpClusterKey := client.ObjectKey{
		Namespace: gcpMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Client.Get(ctx, gcpClusterKey, gcpCluster); err != nil {
		// GCPCluster is not ready
		return nil, nil, nil //nolint:nilerr
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:     r.Client,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return nil, nil, err
	}

	return gcpCluster, clusterScope, nil
}
