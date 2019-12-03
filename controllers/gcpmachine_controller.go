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
	"path"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	gcompute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GCPMachineReconciler reconciles a GCPMachine object
type GCPMachineReconciler struct {
	client.Client
	Log logr.Logger
}

func (r *GCPMachineReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.GCPMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("GCPMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.GCPCluster{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.GCPClusterToGCPMachines)},
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

func (r *GCPMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	logger := r.Log.
		WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("gcpMachine=%s", req.Name))

	// Fetch the GCPMachine instance.
	gcpMachine := &infrav1.GCPMachine{}
	err := r.Get(ctx, req.NamespacedName, gcpMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger = logger.WithName(gcpMachine.APIVersion)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, gcpMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("machine=%s", machine.Name))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	gcpCluster := &infrav1.GCPCluster{}

	gcpClusterName := client.ObjectKey{
		Namespace: gcpMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, gcpClusterName, gcpCluster); err != nil {
		logger.Info("GCPCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("gcpCluster=%s", gcpCluster.Name))

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     r.Client,
		Logger:     logger,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Logger:     logger,
		Client:     r.Client,
		Cluster:    cluster,
		Machine:    machine,
		GCPCluster: gcpCluster,
		GCPMachine: gcpMachine,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !gcpMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineScope, clusterScope)
	}

	// Handle non-deleted machines
	return r.reconcile(ctx, machineScope, clusterScope)
}

func (r *GCPMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling GCPMachine")
	// If the GCPMachine is in an error state, return early.
	if machineScope.GCPMachine.Status.ErrorReason != nil || machineScope.GCPMachine.Status.ErrorMessage != nil {
		machineScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the GCPMachine doesn't have our finalizer, add it.
	if !util.Contains(machineScope.GCPMachine.Finalizers, infrav1.MachineFinalizer) {
		machineScope.GCPMachine.Finalizers = append(machineScope.GCPMachine.Finalizers, infrav1.MachineFinalizer)
	}

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.Data == nil {
		machineScope.Info("Bootstrap data is not yet available")
		return reconcile.Result{}, nil
	}

	computeSvc := compute.NewService(clusterScope)

	// Get or create the instance.
	instance, err := r.getOrCreate(machineScope, computeSvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set an error message if we couldn't find the instance.
	if instance == nil {
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.New("GCE instance cannot be found"))
		return reconcile.Result{}, nil
	}

	// TODO(ncdc): move this validation logic into a validating webhook
	if errs := r.validateUpdate(&machineScope.GCPMachine.Spec, instance); len(errs) > 0 {
		agg := kerrors.NewAggregate(errs)
		record.Warnf(machineScope.GCPMachine, "InvalidUpdate", "Invalid update: %s", agg.Error())
		machineScope.Error(err, "Invalid update")
		return reconcile.Result{}, nil
	}

	// Make sure Spec.ProviderID is always set.
	machineScope.SetProviderID(fmt.Sprintf("gce://%s/%s/%s", clusterScope.Project(), machineScope.Zone(), instance.Name))

	// Proceed to reconcile the GCPMachine state.
	machineScope.SetInstanceStatus(infrav1.InstanceStatus(instance.Status))

	// TODO(vincepri): Remove this annotation when clusterctl is no longer relevant.
	machineScope.SetAnnotation("cluster-api-provider-gcp", "true")

	addrs, err := r.getAddresses(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	machineScope.SetAddresses(addrs)

	switch infrav1.InstanceStatus(instance.Status) {
	case infrav1.InstanceStatusRunning:
		machineScope.Info("Machine instance is running", "instance-id", *machineScope.GetInstanceID())
		machineScope.SetReady()
	case infrav1.InstanceStatusProvisioning, infrav1.InstanceStatusStaging:
		machineScope.Info("Machine instance is pending", "instance-id", *machineScope.GetInstanceID())
	default:
		machineScope.SetErrorReason(capierrors.UpdateMachineError)
		machineScope.SetErrorMessage(errors.Errorf("GCE instance state %q is unexpected", instance.Status))
	}

	if err := r.reconcileLBAttachment(machineScope, clusterScope, instance); err != nil {
		return reconcile.Result{}, errors.Errorf("failed to reconcile LB attachment: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *GCPMachineReconciler) reconcileDelete(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	machineScope.Info("Handling deleted GCPMachine")

	computeSvc := compute.NewService(clusterScope)

	instance, err := r.findInstance(machineScope, computeSvc)
	if err != nil {
		return reconcile.Result{}, err
	}

	defer func() {
		if reterr == nil {
			machineScope.GCPMachine.Finalizers = util.Filter(machineScope.GCPMachine.Finalizers, infrav1.MachineFinalizer)
		}
	}()

	if instance == nil {
		// The machine was never created or was deleted by some other entity
		machineScope.V(3).Info("Unable to locate instance by ID or tags")
		return reconcile.Result{}, nil
	}

	// Check the instance state. If it's already shutting down or terminated,
	// do nothing. Otherwise attempt to delete it.
	switch infrav1.InstanceStatus(instance.Status) {
	case infrav1.InstanceStatusTerminated:
		machineScope.Info("Instance is shutting down or already terminated")
	default:
		machineScope.Info("Terminating instance")
		if err := computeSvc.TerminateInstanceAndWait(machineScope); err != nil {
			record.Warnf(machineScope.GCPMachine, "FailedTerminate", "Failed to terminate instance %q: %v", instance.Name, err)
			return reconcile.Result{}, errors.Errorf("failed to terminate instance: %+v", err)
		}
		record.Eventf(machineScope.GCPMachine, "SuccessfulTerminate", "Terminated instance %q", instance.Name)
	}

	return reconcile.Result{}, nil
}

// findInstance queries the GCP apis and retrieves the instance if it exists, returns nil otherwise.
func (r *GCPMachineReconciler) findInstance(scope *scope.MachineScope, computeSvc *compute.Service) (*gcompute.Instance, error) {
	instance, err := computeSvc.InstanceIfExists(scope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query GCPMachine instance")
	}
	return instance, nil
}

func (r *GCPMachineReconciler) getOrCreate(scope *scope.MachineScope, computeSvc *compute.Service) (*gcompute.Instance, error) {
	instance, err := r.findInstance(scope, computeSvc)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		// Create a new GCPMachine instance if we couldn't find a running instance.
		instance, err = computeSvc.CreateInstance(scope)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create GCPMachine instance")
		}
	}

	return instance, nil
}

func (r *GCPMachineReconciler) getAddresses(instance *gcompute.Instance) ([]v1.NodeAddress, error) {
	addresses := []v1.NodeAddress{}
	for _, nic := range instance.NetworkInterfaces {
		internalAddress := v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: nic.NetworkIP,
		}
		addresses = append(addresses, internalAddress)

		// If access configs are associated with this nic, dig out the external IP
		if len(nic.AccessConfigs) > 0 {
			externalAddress := v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: nic.AccessConfigs[0].NatIP,
			}
			addresses = append(addresses, externalAddress)
		}
	}

	return addresses, nil
}

func (r *GCPMachineReconciler) reconcileLBAttachment(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope, i *gcompute.Instance) error {
	if !machineScope.IsControlPlane() {
		return nil
	}
	computeSvc := compute.NewService(clusterScope)
	groupName := fmt.Sprintf("%s-%s-%s", clusterScope.Name(), infrav1.APIServerRoleTagValue, machineScope.Zone())

	// Get the instance group, or create if necessary.
	group, err := computeSvc.GetOrCreateInstanceGroup(machineScope.Zone(), groupName)
	if err != nil {
		return err
	}

	// Make sure the instance is registered.
	if err := computeSvc.EnsureInstanceGroupMember(machineScope.Zone(), group.Name, i); err != nil {
		return err
	}

	// Update the backend service.
	return computeSvc.UpdateBackendServices()
}

// validateUpdate checks that no immutable fields have been updated and
// returns a slice of errors representing attempts to change immutable state.
func (r *GCPMachineReconciler) validateUpdate(spec *infrav1.GCPMachineSpec, i *gcompute.Instance) (errs []error) {
	// Instance Type
	if spec.InstanceType != path.Base(i.MachineType) {
		errs = append(errs, errors.Errorf("instance type cannot be mutated from %q to %q", i.MachineType, spec.InstanceType))
	}

	// TODO(vincepri): Validate other fields.
	return errs
}

// GCPClusterToGCPMachine is a handler.ToRequestsFunc to be used to enqeue requests for reconciliation
// of GCPMachines.
func (r *GCPMachineReconciler) GCPClusterToGCPMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}

	c, ok := o.Object.(*infrav1.GCPCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a GCPCluster but got a %T", o.Object), "failed to get GCPMachine for GCPCluster")
		return nil
	}
	log := r.Log.WithValues("GCPCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.List(context.TODO(), machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Machines")
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}
