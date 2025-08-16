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

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"

	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"

	// "sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancegroupmanagers"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/instancetemplates"

	// "sigs.k8s.io/cluster-api-provider-gcp/cloud/services"

	// asg "sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/services/autoscaling"
	// "sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/services/ec2"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/logger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
)

// GCPMachinePoolReconciler reconciles a GCPMachinePool object.
type GCPMachinePoolReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	WatchFilterValue string
	// migServiceFactory func(cloud.ClusterScoper) managedinstancegroups.ManagedInstanceGroupInterface
	// ec2ServiceFactory            func(scope.EC2Scope) services.EC2Interface
	// reconcileServiceFactory      func(scope.EC2Scope) services.MachinePoolReconcileInterface
	// objectStoreServiceFactory    func(scope.S3Scope) services.ObjectStoreInterface
	//	TagUnmanagedNetworkResources bool
}

// func (r *GCPMachinePoolReconciler) getManagedInstanceGroupService(scope cloud.ClusterScoper) managedinstancegroups.ManagedInstanceGroupInterface {
// 	if r.migServiceFactory != nil {
// 		return r.migServiceFactory(scope)
// 	}
// 	return managedinstancegroups.New(scope)
// }

// func (r *GCPMachinePoolReconciler) getInstanceTemplatesService(scope cloud.ClusterScoper) instancetemplates.InstanceTemplateInterface {
// 	if r.migServiceFactory != nil {
// 		return r.migServiceFactory(scope)
// 	}
// 	return instancetemplates.New(scope)
// }

// func (r *GCPMachinePoolReconciler) getEC2Service(scope scope.EC2Scope) services.EC2Interface {
// 	if r.ec2ServiceFactory != nil {
// 		return r.ec2ServiceFactory(scope)
// 	}

// 	return ec2.NewService(scope)
// }

// func (r *GCPMachinePoolReconciler) getReconcileService(scope scope.EC2Scope) services.MachinePoolReconcileInterface {
// 	if r.reconcileServiceFactory != nil {
// 		return r.reconcileServiceFactory(scope)
// 	}

// 	return ec2.NewService(scope)
// }

// func (r *GCPMachinePoolReconciler) getObjectStoreService(scope scope.S3Scope) services.ObjectStoreInterface {
// 	if scope.Bucket() == nil {
// 		// S3 bucket usage not enabled, so object store service not needed
// 		return nil
// 	}

// 	if r.objectStoreServiceFactory != nil {
// 		return r.objectStoreServiceFactory(scope)
// 	}

// 	return s3.NewService(scope)
// }

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is the reconciliation loop for GCPMachinePool.
func (r *GCPMachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := logger.FromContext(ctx)

	// Fetch the GCPMachinePool .
	gcpMachinePool := &expinfrav1.GCPMachinePool{}
	err := r.Get(ctx, req.NamespacedName, gcpMachinePool)
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
	clusterObj, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		log.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}
	log = log.WithValues("cluster", klog.KObj(clusterObj))

	if annotations.IsPaused(clusterObj, gcpMachinePool) {
		log.Info("GCPMachinePool or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	gcpCluster := &infrav1.GCPCluster{}
	gcpClusterKey := client.ObjectKey{
		Namespace: gcpMachinePool.Namespace,
		Name:      clusterObj.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, gcpClusterKey, gcpCluster); err != nil {
		log.Info("GCPCluster is not available yet")
		return ctrl.Result{}, nil
	}

	gcpCluster, clusterScope, err := r.getInfraCluster(ctx, log, clusterObj, gcpMachinePool)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting infra provider cluster or control plane object: %w", err)
	}
	if clusterScope == nil {
		log.Info("GCPCluster or GCPManagedControlPlane is not ready yet")
		return ctrl.Result{}, nil
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		ClusterGetter: clusterScope,
		Client:        r.Client,
		// Logger:        log,
		// Cluster:       cluster,
		MachinePool: machinePool,
		// InfraCluster:   infraCluster,
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

	// if feature.Gates.Enabled(feature.MachinePoolMachines) {
	// 	// Patch now so that the status and selectors are available.
	// 	gcpMachinePool.Status.InfrastructureMachineKind = "GCPMachine"
	// 	if err := machinePoolScope.PatchObject(); err != nil {
	// 		return ctrl.Result{}, errors.Wrap(err, "failed to patch GCPMachinePool status")
	// 	}
	// }

	// switch infraScope := clusterScope.(type) {
	// case *scope.ManagedControlPlaneScope:
	// 	if !gcpMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
	// 		return ctrl.Result{}, r.reconcileDelete(ctx, machinePoolScope, infraScope)
	// 	}

	// 	return r.reconcile(ctx, machinePoolScope, infraScope)
	// case *scope.ClusterScope:
	// 	if !gcpMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
	// 		return ctrl.Result{}, r.reconcileDelete(ctx, machinePoolScope, infraScope)
	// 	}

	// 	return r.reconcile(ctx, machinePoolScope, infraScope)
	// default:
	// 	return ctrl.Result{}, errors.New("infraCluster has unknown type")
	// }

	if !gcpMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, machinePoolScope, clusterScope)
	}

	return r.reconcile(ctx, machinePoolScope, clusterScope)
}

func (r *GCPMachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&expinfrav1.GCPMachinePool{}).
		Watches(
			&expclusterv1.MachinePool{},
			handler.EnqueueRequestsFromMapFunc(machinePoolToInfrastructureMapFunc(expinfrav1.GroupVersion.WithKind("GCPMachinePool"))),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), logger.FromContext(ctx).GetLogger(), r.WatchFilterValue)).
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

func (r *GCPMachinePoolReconciler) reconcile(ctx context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	log := logger.FromContext(ctx)

	log.Info("Reconciling GCPMachinePool")

	// if controllerutil.AddFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer) {
	// 	if err := machinePoolScope.PatchObject(ctx); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	// // If the GCPMachine is in an error state, return early.
	// if machinePoolScope.HasFailed() {
	// 	log.Info("Error state detected, skipping reconciliation")

	// 	// TODO: If we are in a failed state, delete the secret regardless of instance state

	// 	return ctrl.Result{}, nil
	// }

	// If the GCPMachinepool doesn't have our finalizer, add it
	if controllerutil.AddFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer) {
		// Register finalizer immediately to avoid orphaning GCP resources
		if err := machinePoolScope.PatchObject(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

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

	// ec2Svc := r.getEC2Service(ec2Scope)
	// asgsvc := r.getManagedInstanceGroupService(clusterScope)
	// reconSvc := r.getReconcileService(ec2Scope)
	// objectStoreSvc := r.getObjectStoreService(s3Scope)

	instanceTemplateKey, err := instancetemplates.New(machinePoolScope).Reconcile(ctx)
	if err != nil {
		log.Error(err, "Error reconciling instanceTemplate")
		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition, expinfrav1.InstanceTemplateNotFoundReason, "%s", err.Error())
		return ctrl.Result{}, err
	}

	// set the InstanceTemplateReadyCondition condition
	conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition)

	if err := instancegroupmanagers.New(machinePoolScope).Reconcile(ctx, instanceTemplateKey); err != nil {
		log.Error(err, "Error reconciling instanceGroupManager")
		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
		return ctrl.Result{}, err
	}

	// set the MIGReadyCondition condition
	conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition)

	// // Find existing MIG
	// mig, err := r.findMIG(machinePoolScope, asgsvc)
	// if err != nil {
	// 	conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
	// 	return ctrl.Result{}, err
	// }

	// canUpdateLaunchTemplate := func() (bool, error) {
	// 	// If there is a change: before changing the template, check if there exist an ongoing instance refresh,
	// 	// because only 1 instance refresh can be "InProgress". If template is updated when refresh cannot be started,
	// 	// that change will not trigger a refresh. Do not start an instance refresh if only userdata changed.
	// 	if mig == nil {
	// 		// If the ASG hasn't been created yet, there is no need to check if we can start the instance refresh.
	// 		// But we want to update the LaunchTemplate because an error in the LaunchTemplate may be blocking the ASG creation.
	// 		return true, nil
	// 	}
	// 	return asgsvc.CanStartASGInstanceRefresh(machinePoolScope)
	// }
	// runPostLaunchTemplateUpdateOperation := func() error {
	// 	// skip instance refresh if MIG is not created yet
	// 	if mig == nil {
	// 		machinePoolScope.Debug("MIG does not exist yet, skipping instance refresh")
	// 		return nil
	// 	}
	// 	// skip instance refresh if explicitly disabled
	// 	if machinePoolScope.GCPMachinePool.Spec.RefreshPreferences != nil && machinePoolScope.GCPMachinePool.Spec.RefreshPreferences.Disable {
	// 		machinePoolScope.Debug("instance refresh disabled, skipping instance refresh")
	// 		return nil
	// 	}
	// 	// After creating a new version of launch template, instance refresh is required
	// 	// to trigger a rolling replacement of all previously launched instances.
	// 	// If ONLY the userdata changed, previously launched instances continue to use the old launch
	// 	// template.
	// 	//
	// 	// FIXME(dlipovetsky,sedefsavas): If the controller terminates, or the StartASGInstanceRefresh returns an error,
	// 	// this conditional will not evaluate to true the next reconcile. If any machines use an older
	// 	// Launch Template version, and the difference between the older and current versions is _more_
	// 	// than userdata, we should start an Instance Refresh.
	// 	machinePoolScope.Info("starting instance refresh", "number of instances", machinePoolScope.MachinePool.Spec.Replicas)
	// 	return asgsvc.StartASGInstanceRefresh(machinePoolScope)
	// }
	// if err := reconSvc.ReconcileLaunchTemplate(ctx, machinePoolScope, machinePoolScope, s3Scope, ec2Svc, objectStoreSvc, canUpdateLaunchTemplate, runPostLaunchTemplateUpdateOperation); err != nil {
	// 	r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedLaunchTemplateReconcile", "Failed to reconcile launch template: %v", err)
	// 	machinePoolScope.Error(err, "failed to reconcile launch template")
	// 	return ctrl.Result{}, err
	// }

	// // set the LaunchTemplateReady condition
	// conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.InstanceTemplateReadyCondition)

	// if mig == nil {
	// 	// Create new MIG
	// 	if err := r.createPool(machinePoolScope, clusterScope); err != nil {
	// 		conditions.MarkFalse(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGProvisionFailedReason, clusterv1.ConditionSeverityError, "%s", err.Error())
	// 		return ctrl.Result{}, err
	// 	}
	// 	return ctrl.Result{
	// 		RequeueAfter: 15 * time.Second,
	// 	}, nil
	// }

	// if feature.Gates.Enabled(feature.MachinePoolMachines) {
	// 	gcpMachineList, err := getGCPMachines(ctx, machinePoolScope.MachinePool, r.Client)
	// 	if err != nil {
	// 		return ctrl.Result{}, err
	// 	}

	// 	if err := createGCPMachinesIfNotExists(ctx, gcpMachineList, machinePoolScope.MachinePool, &machinePoolScope.GCPMachinePool.ObjectMeta, &machinePoolScope.GCPMachinePool.TypeMeta, asg, machinePoolScope.GetLogger(), r.Client, ec2Svc); err != nil {
	// 		machinePoolScope.SetNotReady()
	// 		conditions.MarkFalse(machinePoolScope.GCPMachinePool, clusterv1.ReadyCondition, expinfrav1.GCPMachineCreationFailed, clusterv1.ConditionSeverityWarning, "%s", err.Error())
	// 		return ctrl.Result{}, fmt.Errorf("failed to create gcpmachines: %w", err)
	// 	}

	// 	if err := deleteOrphanedGCPMachines(ctx, gcpMachineList, asg, machinePoolScope.GetLogger(), r.Client); err != nil {
	// 		machinePoolScope.SetNotReady()
	// 		conditions.MarkFalse(machinePoolScope.GCPMachinePool, clusterv1.ReadyCondition, expinfrav1.GCPMachineDeletionFailed, clusterv1.ConditionSeverityWarning, "%s", err.Error())
	// 		return ctrl.Result{}, fmt.Errorf("failed to clean up gcpmachines: %w", err)
	// 	}
	// }

	// if err := r.reconcileLifecycleHooks(ctx, machinePoolScope, asgsvc); err != nil {
	// 	r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedLifecycleHooksReconcile", "Failed to reconcile lifecycle hooks: %v", err)
	// 	return ctrl.Result{}, errors.Wrap(err, "failed to reconcile lifecycle hooks")
	// }

	// if annotations.ReplicasManagedByExternalAutoscaler(machinePoolScope.MachinePool) {
	// 	// Set MachinePool replicas to the ASG DesiredCapacity
	// 	if *machinePoolScope.MachinePool.Spec.Replicas != *mig.DesiredCapacity {
	// 		machinePoolScope.Info("Setting MachinePool replicas to ASG DesiredCapacity",
	// 			"local", machinePoolScope.MachinePool.Spec.Replicas,
	// 			"external", mig.DesiredCapacity)
	// 		machinePoolScope.MachinePool.Spec.Replicas = mig.DesiredCapacity
	// 		if err := machinePoolScope.PatchCAPIMachinePoolObject(ctx); err != nil {
	// 			return ctrl.Result{}, err
	// 		}
	// 	}
	// }

	// if err := r.updatePool(machinePoolScope, clusterScope, mig); err != nil {
	// 	machinePoolScope.Error(err, "error updating GCPMachinePool")
	// 	return ctrl.Result{}, err
	// }

	// instanceTemplateID := machinePoolScope.GetInstanceTemplateIDStatus()
	// asgName := machinePoolScope.Name()
	// resourceServiceToUpdate := []scope.ResourceServiceToUpdate{
	// 	{
	// 		ResourceID:      &instanceTemplateID,
	// 		ResourceService: ec2Svc,
	// 	},
	// 	{
	// 		ResourceID:      &asgName,
	// 		ResourceService: asgsvc,
	// 	},
	// }
	// err = reconSvc.ReconcileTags(machinePoolScope, resourceServiceToUpdate)
	// if err != nil {
	// 	return ctrl.Result{}, errors.Wrap(err, "error updating tags")
	// }

	// // Make sure Spec.ProviderID is always set.
	// machinePoolScope.GCPMachinePool.Spec.ProviderID = mig.ID
	// providerIDList := make([]string, len(mig.Instances))

	// for i, gceInstance := range mig.Instances {
	// 	providerIDList[i] = fmt.Sprintf("gcp:///%s/%s", gceInstance.AvailabilityZone, gceInstance.ID)
	// }

	// machinePoolScope.SetAnnotation("cluster-api-provider-gcp", "true")

	// machinePoolScope.GCPMachinePool.Spec.ProviderIDList = providerIDList
	// machinePoolScope.GCPMachinePool.Status.Replicas = int32(len(providerIDList)) //#nosec G115
	// machinePoolScope.GCPMachinePool.Status.Ready = true
	// conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition)

	// err = machinePoolScope.UpdateInstanceStatuses(ctx, mig.Instances)
	// if err != nil {
	// 	machinePoolScope.Error(err, "failed updating instances", "instances", mig.Instances)
	// }

	// if feature.Gates.Enabled(feature.MachinePoolMachines) {
	// 	return ctrl.Result{
	// 		// Regularly update `GCPMachine` objects, for example if ASG was scaled or refreshed instances
	// 		// TODO: Requeueing interval can be removed or prolonged once reconciliation of ASG EC2 instances
	// 		//       can be triggered by events (e.g. with feature gate `EventBridgeInstanceState`).
	// 		//       See https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues/5323.
	// 		RequeueAfter: 3 * time.Minute,
	// 	}, nil
	// }

	return ctrl.Result{}, nil
}

func (r *GCPMachinePoolReconciler) reconcileDelete(ctx context.Context, machinePoolScope *scope.MachinePoolScope, infraScope *scope.ClusterScope) error {
	log := logger.FromContext(ctx)

	log.Info("Handling deleted GCPMachinePool")

	// if feature.Gates.Enabled(feature.MachinePoolMachines) {
	// 	if err := reconcileDeleteGCPMachines(ctx, machinePoolScope.MachinePool, r.Client, machinePoolScope.GetLogger()); err != nil {
	// 		return err
	// 	}
	// }

	// instanceTemplatesSvc := r.getInstanceTemplatesService(ec2Scope)
	// migSvc := r.getManagedInstanceGroupService(clusterScope)

	if err := instancegroupmanagers.New(machinePoolScope).Delete(ctx); err != nil {
		log.Error(err, "Error deleting instanceGroupManager")
		r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instancegroupmanager: %v", err)

		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
		return err
	}

	// // set the MIGReadyCondition condition
	// conditions.MarkTrue(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition)

	// mig, err := r.findMIG(machinePoolScope, migSvc)
	// if err != nil {
	// 	return err
	// }

	// if mig == nil {
	// 	machinePoolScope.Warn("Unable to locate MIG")
	// 	r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeNormal, expinfrav1.MIGNotFoundReason, "Unable to find matching MIG")
	// } else {
	// 	machinePoolScope.SetMIGStatus(mig.Status)
	// 	switch mig.Status {
	// 	case expinfrav1.MIGStatusDeleteInProgress:
	// 		// MIG is already deleting
	// 		machinePoolScope.SetNotReady()
	// 		conditions.MarkFalse(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
	// 		r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "DeletionInProgress", "MIG deletion in progress: %q", mig.Name)
	// 		machinePoolScope.Info("MIG is already deleting", "name", mig.Name)
	// 	default:
	// 		machinePoolScope.Info("Deleting MIG", "id", mig.Name, "status", mig.Status)
	// 		if err := migSvc.DeleteMIGAndWait(mig.Name); err != nil {
	// 			r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete MIG %q: %v", mig.Name, err)
	// 			return errors.Wrap(err, "failed to delete MIG")
	// 		}
	// 	}
	// }

	// instanceTemplateID := machinePoolScope.GCPMachinePool.Status.InstanceTemplateID
	// instanceTemplate, _, _, _, err := instanceTemplatesSvc.GetInstanceTemplate(machinePoolScope.LaunchTemplateName()) //nolint:dogsled
	// if err != nil {
	// 	return err
	// }

	// if instanceTemplate == nil {
	// 	machinePoolScope.Debug("Unable to locate instance template")
	// 	r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeNormal, expinfrav1.MIGNotFoundReason, "Unable to find matching MIG")
	// 	controllerutil.RemoveFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer)
	// 	return nil
	// }

	// machinePoolScope.Info("deleting instance template", "name", instanceTemplate.Name)
	// if err := instanceTemplatesSvc.DeleteLaunchTemplate(instanceTemplateID); err != nil {
	// 	r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instance template %q: %v", instanceTemplate.Name, err)
	// 	return errors.Wrap(err, "failed to delete instance template")
	// }

	// machinePoolScope.Info("successfully deleted ManagedInstanceGroup and InstanceTemplate")

	if err := instancetemplates.New(machinePoolScope).Delete(ctx); err != nil {
		log.Error(err, "Error deleting instanceTemplates")
		r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete instance template: %v", err)

		// record.Warnf(machineScope.GCPMachine, "GCPMachineReconcile", "Reconcile error - %v", err)
		// conditions.MarkUnknown(machinePoolScope.GCPMachinePool, expinfrav1.MIGReadyCondition, expinfrav1.MIGNotFoundReason, "%s", err.Error())
		return err
	}

	// remove finalizer
	controllerutil.RemoveFinalizer(machinePoolScope.GCPMachinePool, expinfrav1.MachinePoolFinalizer)

	return nil
}

// func (r *GCPMachinePoolReconciler) updatePool(machinePoolScope *scope.MachinePoolScope, clusterScope cloud.ClusterScoper, existingMIG *expinfrav1.ManagedInstanceGroup) error {
// 	migSvc := r.getManagedInstanceGroupService(clusterScope)

// 	subnetIDs, err := migSvc.SubnetIDs(machinePoolScope)
// 	if err != nil {
// 		return errors.Wrapf(err, "fail to get subnets for MIG")
// 	}
// 	machinePoolScope.Debug("determining if subnets change in machinePoolScope",
// 		"subnets of machinePoolScope", subnetIDs,
// 		"subnets of existing mig", existingMIG.Subnets)
// 	less := func(a, b string) bool { return a < b }
// 	subnetDiff := cmp.Diff(subnetIDs, existingMIG.Subnets, cmpopts.SortSlices(less))
// 	if subnetDiff != "" {
// 		machinePoolScope.Debug("mig subnet diff detected", "diff", subnetDiff)
// 	}

// 	migDiff := diffMIG(machinePoolScope, existingMIG)
// 	if migDiff != "" {
// 		machinePoolScope.Debug("mig diff detected", "migDiff", migDiff, "subnetDiff", subnetDiff)
// 	}
// 	if migDiff != "" || subnetDiff != "" {
// 		machinePoolScope.Info("updating ManagedInstanceGroup")

// 		if err := migSvc.UpdateMIG(machinePoolScope); err != nil {
// 			r.Recorder.Eventf(machinePoolScope.GCPMachinePool, corev1.EventTypeWarning, "FailedUpdate", "Failed to update MIG: %v", err)
// 			return errors.Wrap(err, "unable to update MIG")
// 		}
// 	}

// 	suspendedProcessesSlice := machinePoolScope.GCPMachinePool.Spec.SuspendProcesses.ConvertSetValuesToStringSlice()
// 	if !cmp.Equal(existingMIG.CurrentlySuspendProcesses, suspendedProcessesSlice) {
// 		clusterScope.Info("reconciling processes", "suspend-processes", suspendedProcessesSlice)
// 		var (
// 			toBeSuspended []string
// 			toBeResumed   []string

// 			currentlySuspended = make(map[string]struct{})
// 			desiredSuspended   = make(map[string]struct{})
// 		)

// 		// Convert the items to a map, so it's easy to create an effective diff from these two slices.
// 		for _, p := range existingMIG.CurrentlySuspendProcesses {
// 			currentlySuspended[p] = struct{}{}
// 		}

// 		for _, p := range suspendedProcessesSlice {
// 			desiredSuspended[p] = struct{}{}
// 		}

// 		// Anything that remains in the desired items is not currently suspended so must be suspended.
// 		// Anything that remains in the currentlySuspended list must be resumed since they were not part of
// 		// desiredSuspended.
// 		for k := range desiredSuspended {
// 			if _, ok := currentlySuspended[k]; ok {
// 				delete(desiredSuspended, k)
// 			}
// 			delete(currentlySuspended, k)
// 		}

// 		// Convert them back into lists to pass them to resume/suspend.
// 		for k := range desiredSuspended {
// 			toBeSuspended = append(toBeSuspended, k)
// 		}

// 		for k := range currentlySuspended {
// 			toBeResumed = append(toBeResumed, k)
// 		}

// 		if len(toBeSuspended) > 0 {
// 			clusterScope.Info("suspending processes", "processes", toBeSuspended)
// 			if err := migSvc.SuspendProcesses(existingMIG.Name, toBeSuspended); err != nil {
// 				return errors.Wrapf(err, "failed to suspend processes while trying update pool")
// 			}
// 		}
// 		if len(toBeResumed) > 0 {
// 			clusterScope.Info("resuming processes", "processes", toBeResumed)
// 			if err := migSvc.ResumeProcesses(existingMIG.Name, toBeResumed); err != nil {
// 				return errors.Wrapf(err, "failed to resume processes while trying update pool")
// 			}
// 		}
// 	}
// 	return nil
// }

// func (r *GCPMachinePoolReconciler) createPool(machinePoolScope *scope.MachinePoolScope, clusterScope cloud.ClusterScoper) error {
// 	clusterScope.Info("Initializing MIG client")

// 	migSvc := r.getManagedInstanceGroupService(clusterScope)

// 	machinePoolScope.Info("Creating Managed Instance Group")
// 	if _, err := migSvc.CreateMIG(machinePoolScope); err != nil {
// 		return errors.Wrapf(err, "failed to create Managed Instance Group")
// 	}

// 	return nil
// }

// func (r *GCPMachinePoolReconciler) findMIG(machinePoolScope *scope.MachinePoolScope, migSvc services.MIGInterface) (*expinfrav1.ManagedInstanceGroup, error) {
// 	// Query the instance using tags.
// 	mig, err := migSvc.GetMIGByName(machinePoolScope)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to query Managed Instance Group by name")
// 	}

// 	return mig, nil
// }

// // diffMIG compares incoming GCPMachinePool and compares against existing MIG.
// func diffMIG(machinePoolScope *scope.MachinePoolScope, existingMIG *expinfrav1.ManagedInstanceGroup) string {
// 	detectedMachinePoolSpec := machinePoolScope.MachinePool.Spec.DeepCopy()

// 	if !annotations.ReplicasManagedByExternalAutoscaler(machinePoolScope.MachinePool) {
// 		detectedMachinePoolSpec.Replicas = existingMIG.DesiredCapacity
// 	}
// 	if diff := cmp.Diff(machinePoolScope.MachinePool.Spec, *detectedMachinePoolSpec); diff != "" {
// 		return diff
// 	}

// 	detectedGCPMachinePoolSpec := machinePoolScope.GCPMachinePool.Spec.DeepCopy()
// 	detectedGCPMachinePoolSpec.MaxSize = existingMIG.MaxSize
// 	detectedGCPMachinePoolSpec.MinSize = existingMIG.MinSize
// 	detectedGCPMachinePoolSpec.CapacityRebalance = existingMIG.CapacityRebalance
// 	{
// 		mixedInstancesPolicy := machinePoolScope.GCPMachinePool.Spec.MixedInstancesPolicy
// 		// InstancesDistribution is optional, and the default values come from GCP, so
// 		// they are not set by the GCPMachinePool defaulting webhook. If InstancesDistribution is
// 		// not set, we use the GCP values for the purpose of comparison.
// 		if mixedInstancesPolicy != nil && mixedInstancesPolicy.InstancesDistribution == nil {
// 			mixedInstancesPolicy = machinePoolScope.GCPMachinePool.Spec.MixedInstancesPolicy.DeepCopy()
// 			mixedInstancesPolicy.InstancesDistribution = existingMIG.MixedInstancesPolicy.InstancesDistribution
// 		}

// 		if !cmp.Equal(mixedInstancesPolicy, existingMIG.MixedInstancesPolicy) {
// 			detectedGCPMachinePoolSpec.MixedInstancesPolicy = existingMIG.MixedInstancesPolicy
// 		}
// 	}

// 	return cmp.Diff(machinePoolScope.GCPMachinePool.Spec, *detectedGCPMachinePoolSpec)
// }

// // getOwnerMachinePool returns the MachinePool object owning the current resource.
// func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*expclusterv1.MachinePool, error) {
// 	for _, ref := range obj.OwnerReferences {
// 		if ref.Kind != "MachinePool" {
// 			continue
// 		}
// 		gv, err := schema.ParseGroupVersion(ref.APIVersion)
// 		if err != nil {
// 			return nil, errors.WithStack(err)
// 		}
// 		if gv.Group == expclusterv1.GroupVersion.Group {
// 			return getMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
// 		}
// 	}
// 	return nil, nil
// }

// // getMachinePoolByName finds and return a Machine object using the specified params.
// func getMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*expclusterv1.MachinePool, error) {
// 	m := &expclusterv1.MachinePool{}
// 	key := client.ObjectKey{Name: name, Namespace: namespace}
// 	if err := c.Get(ctx, key, m); err != nil {
// 		return nil, err
// 	}
// 	return m, nil
// }

// func machinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.MapFunc {
// 	return func(ctx context.Context, o client.Object) []reconcile.Request {
// 		m, ok := o.(*expclusterv1.MachinePool)
// 		if !ok {
// 			klog.Errorf("Expected a MachinePool but got a %T", o)
// 		}

// 		gk := gvk.GroupKind()
// 		// Return early if the GroupKind doesn't match what we expect
// 		infraGK := m.Spec.Template.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
// 		if gk != infraGK {
// 			return nil
// 		}

// 		return []reconcile.Request{
// 			{
// 				NamespacedName: client.ObjectKey{
// 					Namespace: m.Namespace,
// 					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
// 				},
// 			},
// 		}
// 	}
// }

// // reconcileLifecycleHooks periodically reconciles a lifecycle hook for the ASG.
// func (r *GCPMachinePoolReconciler) reconcileLifecycleHooks(ctx context.Context, machinePoolScope *scope.MachinePoolScope, asgsvc services.ASGInterface) error {
// 	asgName := machinePoolScope.Name()

// 	return asg.ReconcileLifecycleHooks(ctx, asgsvc, asgName, machinePoolScope.GetLifecycleHooks(), map[string]bool{}, machinePoolScope.GetMachinePool(), machinePoolScope)
// }

func (r *GCPMachinePoolReconciler) getInfraCluster(ctx context.Context, log *logger.Logger, cluster *clusterv1.Cluster, gcpMachinePool *expinfrav1.GCPMachinePool) (*infrav1.GCPCluster, *scope.ClusterScope, error) {
	// var clusterScope *scope.ClusterScope
	// var managedControlPlaneScope *scope.ManagedControlPlaneScope
	// var err error

	// if cluster.Spec.ControlPlaneRef != nil && cluster.Spec.ControlPlaneRef.Kind == controllers.GCPManagedControlPlaneRefKind {
	// 	controlPlane := &expinfrav1.GCPManagedControlPlane{}
	// 	controlPlaneName := client.ObjectKey{
	// 		Namespace: gcpMachinePool.Namespace,
	// 		Name:      cluster.Spec.ControlPlaneRef.Name,
	// 	}

	// 	if err := r.Get(ctx, controlPlaneName, controlPlane); err != nil {
	// 		// GCPManagedControlPlane is not ready
	// 		return nil, nil, nil //nolint:nilerr
	// 	}

	// 	managedControlPlaneScope, err = scope.NewManagedControlPlaneScope(scope.ManagedControlPlaneScopeParams{
	// 		Client:                       r.Client,
	// 		Logger:                       log,
	// 		Cluster:                      cluster,
	// 		ControlPlane:                 controlPlane,
	// 		ControllerName:               "gcpManagedControlPlane",
	// 		TagUnmanagedNetworkResources: r.TagUnmanagedNetworkResources,
	// 	})
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}

	// 	return managedControlPlaneScope, managedControlPlaneScope, nil
	// }

	gcpCluster := &infrav1.GCPCluster{}

	infraClusterName := client.ObjectKey{
		Namespace: gcpMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err := r.Client.Get(ctx, infraClusterName, gcpCluster); err != nil {
		// GCPCluster is not ready
		return nil, nil, nil //nolint:nilerr
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client: r.Client,
		// Logger:     log,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
		// ControllerName:               "gcpmachine",
		// TagUnmanagedNetworkResources: r.TagUnmanagedNetworkResources,
	})
	if err != nil {
		return nil, nil, err
	}

	return gcpCluster, clusterScope, nil
}
