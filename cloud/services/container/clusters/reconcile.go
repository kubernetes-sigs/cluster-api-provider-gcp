/*
Copyright 2023 The Kubernetes Authors.

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

package clusters

import (
	"context"
	"fmt"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/shared"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconcile GKE cluster.
func (s *Service) Reconcile(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("service", "container.clusters")
	log.Info("Reconciling cluster resources")

	cluster, err := s.describeCluster(ctx, &log)
	if err != nil {
		s.scope.GCPManagedControlPlane.Status.Initialized = false
		s.scope.GCPManagedControlPlane.Status.Ready = false
		conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster not found, creating")
		s.scope.GCPManagedControlPlane.Status.Initialized = false
		s.scope.GCPManagedControlPlane.Status.Ready = false

		nodePools, _, err := s.scope.GetAllNodePools(ctx)
		if err != nil {
			conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		if s.scope.IsAutopilotCluster() {
			if len(nodePools) > 0 {
				log.Error(ErrAutopilotClusterMachinePoolsNotAllowed, fmt.Sprintf("%d machine pools defined", len(nodePools)))
				conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				return ctrl.Result{}, ErrAutopilotClusterMachinePoolsNotAllowed
			}
		} else {
			if len(nodePools) == 0 {
				log.Info("At least 1 node pool is required to create GKE cluster with autopilot disabled")
				conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneRequiresAtLeastOneNodePoolReason, clusterv1.ConditionSeverityInfo, "")
				return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
			}
		}

		if err = s.createCluster(ctx, &log); err != nil {
			log.Error(err, "failed creating cluster")
			conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		log.Info("Cluster created provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition)
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	}

	log.V(2).Info("gke cluster found", "status", cluster.Status)
	s.scope.GCPManagedControlPlane.Status.CurrentVersion = cluster.CurrentMasterVersion

	switch cluster.Status {
	case containerpb.Cluster_PROVISIONING:
		log.Info("Cluster provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition)
		s.scope.GCPManagedControlPlane.Status.Initialized = false
		s.scope.GCPManagedControlPlane.Status.Ready = false
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	case containerpb.Cluster_RECONCILING:
		log.Info("Cluster reconciling in progress")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneUpdatingCondition)
		s.scope.GCPManagedControlPlane.Status.Initialized = true
		s.scope.GCPManagedControlPlane.Status.Ready = true
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	case containerpb.Cluster_STOPPING:
		log.Info("Cluster stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)
		s.scope.GCPManagedControlPlane.Status.Initialized = false
		s.scope.GCPManagedControlPlane.Status.Ready = false
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	case containerpb.Cluster_ERROR, containerpb.Cluster_DEGRADED:
		var msg string
		if len(cluster.Conditions) > 0 {
			msg = cluster.Conditions[0].GetMessage()
		}
		log.Error(errors.New("Cluster in error/degraded state"), msg, "name", s.scope.ClusterName())
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneErrorReason, clusterv1.ConditionSeverityError, "")
		s.scope.GCPManagedControlPlane.Status.Ready = false
		s.scope.GCPManagedControlPlane.Status.Initialized = false
		return ctrl.Result{}, nil
	case containerpb.Cluster_RUNNING:
		log.Info("Cluster running")
	default:
		statusErr := NewErrUnexpectedClusterStatus(string(cluster.Status))
		log.Error(statusErr, fmt.Sprintf("Unhandled cluster status %s", cluster.Status), "name", s.scope.ClusterName())
		return ctrl.Result{}, statusErr
	}

	// Check for cluster diffs and update
	isUpdating, err := s.checkDiffAndUpdateCluster(ctx, cluster, &log)
	if err != nil {
		log.Error(err, "failed to check diff and update cluster")
		return ctrl.Result{}, err
	}
	if isUpdating {
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneUpdatingCondition)
		s.scope.GCPManagedControlPlane.Status.Initialized = true
		s.scope.GCPManagedControlPlane.Status.Ready = true
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	}

	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneUpdatingCondition, infrav1exp.GKEControlPlaneUpdatedReason, clusterv1.ConditionSeverityInfo, "")

	// Reconcile kubeconfig
	err = s.reconcileKubeconfig(ctx, cluster, &log)
	if err != nil {
		log.Error(err, "Failed to reconcile CAPI kubeconfig")
		return ctrl.Result{}, err
	}
	err = s.reconcileAdditionalKubeconfigs(ctx, cluster, &log)
	if err != nil {
		log.Error(err, "Failed to reconcile additional kubeconfig")
		return ctrl.Result{}, err
	}

	s.scope.SetEndpoint(cluster.Endpoint)
	conditions.MarkTrue(s.scope.ConditionSetter(), clusterv1.ReadyCondition)
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition)
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneCreatedReason, clusterv1.ConditionSeverityInfo, "")
	s.scope.GCPManagedControlPlane.Status.Ready = true
	s.scope.GCPManagedControlPlane.Status.Initialized = true

	log.Info("Cluster reconciled")

	return ctrl.Result{}, nil
}

// Delete delete GKE cluster.
func (s *Service) Delete(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("service", "container.clusters")
	log.Info("Deleting cluster resources")

	cluster, err := s.describeCluster(ctx, &log)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster already deleted")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition, infrav1exp.GKEControlPlaneDeletedReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	switch cluster.Status {
	case containerpb.Cluster_PROVISIONING:
		log.Info("Cluster provisioning in progress")
		return ctrl.Result{}, nil
	case containerpb.Cluster_RECONCILING:
		log.Info("Cluster reconciling in progress")
		return ctrl.Result{}, nil
	case containerpb.Cluster_STOPPING:
		log.Info("Cluster stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)
		return ctrl.Result{}, nil
	default:
		break
	}

	if err = s.deleteCluster(ctx, &log); err != nil {
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	log.Info("Cluster deleting in progress")
	s.scope.GCPManagedControlPlane.Status.Initialized = false
	s.scope.GCPManagedControlPlane.Status.Ready = false
	conditions.MarkFalse(s.scope.ConditionSetter(), clusterv1.ReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)

	return ctrl.Result{}, nil
}

func (s *Service) describeCluster(ctx context.Context, log *logr.Logger) (*containerpb.Cluster, error) {
	getClusterRequest := &containerpb.GetClusterRequest{
		Name: s.scope.ClusterFullName(),
	}
	cluster, err := s.scope.ManagedControlPlaneClient().GetCluster(ctx, getClusterRequest)
	if err != nil {
		var e *apierror.APIError
		if ok := errors.As(err, &e); ok {
			if e.GRPCStatus().Code() == codes.NotFound {
				return nil, nil
			}
		}
		log.Error(err, "Error getting GKE cluster", "name", s.scope.ClusterName())
		return nil, err
	}

	return cluster, nil
}

func (s *Service) createCluster(ctx context.Context, log *logr.Logger) error {
	nodePools, machinePools, _ := s.scope.GetAllNodePools(ctx)

	log.V(2).Info("Running pre-flight checks on machine pools before cluster creation")
	if err := shared.ManagedMachinePoolsPreflightCheck(nodePools, machinePools, s.scope.Region()); err != nil {
		return fmt.Errorf("preflight checks on machine pools before cluster create: %w", err)
	}

	isRegional := shared.IsRegional(s.scope.Region())

	cluster := &containerpb.Cluster{
		Name:    s.scope.ClusterName(),
		Network: *s.scope.GCPManagedCluster.Spec.Network.Name,
		Autopilot: &containerpb.Autopilot{
			Enabled: s.scope.GCPManagedControlPlane.Spec.EnableAutopilot,
		},
		ReleaseChannel: &containerpb.ReleaseChannel{
			Channel: infrav1exp.ConvertToSdkReleaseChannel(s.scope.GCPManagedControlPlane.Spec.ReleaseChannel),
		},
		ResourceLabels:                 s.scope.GCPManagedControlPlane.Spec.ResourceLabels,
		AddonsConfig:                   infrav1exp.ConvertToSdkAddonsConfig(s.scope.GCPManagedControlPlane.Spec.AddonsConfig),
		LoggingConfig:                  infrav1exp.ConvertToSdkLoggingConfig(s.scope.GCPManagedControlPlane.Spec.LoggingConfig),
		MasterAuthorizedNetworksConfig: infrav1exp.ConvertToSdkMasterAuthorizedNetworksConfig(s.scope.GCPManagedControlPlane.Spec.MasterAuthorizedNetworksConfig),
		ShieldedNodes:                  infrav1exp.ConvertToSdkShieldedNodes(s.scope.GCPManagedControlPlane.Spec.ShieldedNodes),
	}
	if s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion != nil {
		cluster.InitialClusterVersion = *s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion
	}
	if !s.scope.IsAutopilotCluster() {
		cluster.NodePools = scope.ConvertToSdkNodePools(nodePools, machinePools, isRegional)
	}

	if s.scope.GCPManagedControlPlane.Spec.ClusterIpv4Cidr != nil {
		cluster.ClusterIpv4Cidr = *s.scope.GCPManagedControlPlane.Spec.ClusterIpv4Cidr
	}

	if s.scope.GCPManagedControlPlane.Spec.IPAllocationPolicy != nil {
		cluster.IpAllocationPolicy = infrav1exp.ConvertToSdkIPAllocationPolicy(s.scope.GCPManagedControlPlane.Spec.IPAllocationPolicy)
	}

	if s.scope.GCPManagedControlPlane.Spec.MaintenancePolicy != nil {
		mp, err := infrav1exp.ConvertToSdkMaintenancePolicy(s.scope.GCPManagedControlPlane.Spec.MaintenancePolicy)
		if err != nil {
			return fmt.Errorf("invalid conversion to SDK MaintenancePolicy: %w", err)
		}
		cluster.MaintenancePolicy = mp
	}

	if s.scope.GCPManagedControlPlane.Spec.NetworkConfig != nil {
		cluster.NetworkConfig = infrav1exp.ConvertToSdkNetworkConfig(s.scope.GCPManagedControlPlane.Spec.NetworkConfig)
	}

	if s.scope.GCPManagedControlPlane.Spec.DefaultMaxPodsConstraint != nil {
		cluster.DefaultMaxPodsConstraint = &containerpb.MaxPodsConstraint{
			MaxPodsPerNode: s.scope.GCPManagedControlPlane.Spec.DefaultMaxPodsConstraint.MaxPodsPerNode,
		}
	}

	if s.scope.GCPManagedControlPlane.Spec.PrivateClusterConfig != nil {
		cluster.PrivateClusterConfig = infrav1exp.ConvertToSdkPrivateClusterConfig(s.scope.GCPManagedControlPlane.Spec.PrivateClusterConfig)
	}

	if s.scope.GCPManagedControlPlane.Spec.WorkloadIdentityConfig != nil {
		cluster.WorkloadIdentityConfig = infrav1exp.ConvertToSdkWorkloadIdentityConfig(s.scope.GCPManagedControlPlane.Spec.WorkloadIdentityConfig)
	}

	createClusterRequest := &containerpb.CreateClusterRequest{
		Cluster: cluster,
		Parent:  s.scope.ClusterLocation(),
	}

	log.V(2).Info("Creating GKE cluster")
	_, err := s.scope.ManagedControlPlaneClient().CreateCluster(ctx, createClusterRequest)
	if err != nil {
		log.Error(err, "Error creating GKE cluster", "name", s.scope.ClusterName())
		return err
	}

	return nil
}

func (s *Service) updateCluster(ctx context.Context, updateClusterRequest *containerpb.UpdateClusterRequest, log *logr.Logger) error {
	_, err := s.scope.ManagedControlPlaneClient().UpdateCluster(ctx, updateClusterRequest)
	if err != nil {
		log.Error(err, "Error updating GKE cluster", "name", s.scope.ClusterName())
		return err
	}

	return nil
}

func (s *Service) updateMaintenancePolicy(ctx context.Context, updateMaintenancePolicyRequest *containerpb.SetMaintenancePolicyRequest, log *logr.Logger) error {
	_, err := s.scope.ManagedControlPlaneClient().SetMaintenancePolicy(ctx, updateMaintenancePolicyRequest)
	if err != nil {
		log.Error(err, "Error updating MaintenancePolicy", "name", s.scope.ClusterName())
		return err
	}

	return nil
}

func (s *Service) deleteCluster(ctx context.Context, log *logr.Logger) error {
	deleteClusterRequest := &containerpb.DeleteClusterRequest{
		Name: s.scope.ClusterFullName(),
	}
	_, err := s.scope.ManagedControlPlaneClient().DeleteCluster(ctx, deleteClusterRequest)
	if err != nil {
		log.Error(err, "Error deleting GKE cluster", "name", s.scope.ClusterName())
		return err
	}

	return nil
}

// checkDiffAndUpdateCluster sequentially checks different cluster fields diffs and submits an update request if necessary.
// Updates are done sequentially because the SDK does not allow updating multiple fields at once.
func (s *Service) checkDiffAndUpdateCluster(ctx context.Context, existingCluster *containerpb.Cluster, log *logr.Logger) (bool, error) {
	log.V(4).Info("Checking diff and preparing update.")

	needUpdateReleaseChannel, updateReleaseChannelRequest := s.checkDiffAndPrepareUpdateReleaseChannel(existingCluster, log)
	if needUpdateReleaseChannel {
		log.Info("release channel update required")
		err := s.updateCluster(ctx, updateReleaseChannelRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster release channel updating in progress")
		return true, nil
	}

	needUpdateMasterVersion, updateMasterVersionRequest := s.checkDiffAndPrepareUpdateMasterVersion(existingCluster, log)
	if needUpdateMasterVersion {
		log.Info("master version update required")
		err := s.updateCluster(ctx, updateMasterVersionRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster master version updating in progress")
		return true, nil
	}

	needUpdateMasterAuthorizedNetworkConfigs, updateMasterAuthorizedNetworkConfigsRequest := s.checkDiffAndPrepareUpdateMasterAuthorizedNetworkConfigs(existingCluster, log)
	if needUpdateMasterAuthorizedNetworkConfigs {
		log.Info("master authorized network configs update required")
		err := s.updateCluster(ctx, updateMasterAuthorizedNetworkConfigsRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster master authorized network configs updating in progress")
		return true, nil
	}

	needUpdateAddonsConfig, updateAddonsConfigRequest := s.checkDiffAndPrepareUpdateAddonsConfig(existingCluster, log)
	if needUpdateAddonsConfig {
		log.Info("addons config update required")
		err := s.updateCluster(ctx, updateAddonsConfigRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster addons configs updating in progress")
		return true, nil
	}

	needUpdateDNSConfig, updateDNSConfigRequest := s.checkDiffAndPrepareUpdateDNSConfig(existingCluster, log)
	if needUpdateDNSConfig {
		log.Info("DNSConfig update required")
		err := s.updateCluster(ctx, updateDNSConfigRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster DNSConfig updating in progress")
		return true, nil
	}

	needUpdateMasterGlobalAccessEnabled, updateMasterGlobalAccessEnabledRequest := s.checkDiffAndPrepareUpdateMasterGlobalAccessEnabled(existingCluster, log)
	if needUpdateMasterGlobalAccessEnabled {
		log.Info("master global access config update required")
		err := s.updateCluster(ctx, updateMasterGlobalAccessEnabledRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster master global access config updating in progress")
		return true, nil
	}

	needUpdateShieldedNodes, updateShieldedNodesRequest := s.checkDiffAndPrepareUpdateShieldedNodes(existingCluster, log)
	if needUpdateShieldedNodes {
		log.Info("shielded nodes update required")
		err := s.updateCluster(ctx, updateShieldedNodesRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster shielded nodes updating in progress")
		return true, nil
	}

	needUpdateWorkloadIdentityConfig, updateWorkloadIdentityConfigRequest := s.checkDiffAndPrepareUpdateWorkloadIdentityConfig(existingCluster, log)
	if needUpdateWorkloadIdentityConfig {
		log.Info("workload identity config update required")
		err := s.updateCluster(ctx, updateWorkloadIdentityConfigRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster workload identity config updating in progress")
		return true, nil
	}

	needUpdateLoggingConfig, updateLoggingConfigRequest := s.checkDiffAndPrepareUpdateLoggingConfig(existingCluster, log)
	if needUpdateLoggingConfig {
		log.Info("logging config update required")
		err := s.updateCluster(ctx, updateLoggingConfigRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster logging config updating in progress")
		return true, nil
	}

	needUpdateMaintenancePolicy, updateMaintenancePolicyRequest, err := s.checkDiffAndPrepareUpdateMaintenancePolicy(existingCluster, log)
	if err != nil {
		log.Error(err, "failed to check maintenance policy diff")
		return false, err
	}
	if needUpdateMaintenancePolicy {
		log.Info("MaintenancePolicy update required")
		err := s.updateMaintenancePolicy(ctx, updateMaintenancePolicyRequest, log)
		if err != nil {
			return false, err
		}
		log.Info("cluster MaintenancePolicy updating in progress")
		return true, nil
	}
	// No updates needed
	return false, nil
}

func (s *Service) checkDiffAndPrepareUpdateReleaseChannel(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredReleaseChannel := infrav1exp.ConvertToSdkReleaseChannel(s.scope.GCPManagedControlPlane.Spec.ReleaseChannel)
	if desiredReleaseChannel != existingCluster.ReleaseChannel.Channel {
		log.V(2).Info("Release channel update required", "current", existingCluster.ReleaseChannel.Channel, "desired", desiredReleaseChannel)
		needUpdate = true
		clusterUpdate.DesiredReleaseChannel = &containerpb.ReleaseChannel{
			Channel: desiredReleaseChannel,
		}
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("ReleaseChannel update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateMasterVersion(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	// Master version
	if s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion != nil {
		desiredMasterVersion := *s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion
		if desiredMasterVersion != existingCluster.InitialClusterVersion {
			needUpdate = true
			clusterUpdate.DesiredMasterVersion = desiredMasterVersion
			log.V(2).Info("Master version update required", "current", existingCluster.InitialClusterVersion, "desired", desiredMasterVersion)
		}
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("MasterVersion update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateMasterAuthorizedNetworkConfigs(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	// When desiredMasterAuthorizedNetworksConfig is nil, it means that the user wants to disable the feature.
	desiredMasterAuthorizedNetworksConfig := infrav1exp.ConvertToSdkMasterAuthorizedNetworksConfig(s.scope.GCPManagedControlPlane.Spec.MasterAuthorizedNetworksConfig)
	if !compareMasterAuthorizedNetworksConfig(desiredMasterAuthorizedNetworksConfig, existingCluster.MasterAuthorizedNetworksConfig) {
		needUpdate = true
		clusterUpdate.DesiredMasterAuthorizedNetworksConfig = desiredMasterAuthorizedNetworksConfig
		log.V(2).Info("Master authorized networks config update required", "current", existingCluster.MasterAuthorizedNetworksConfig, "desired", desiredMasterAuthorizedNetworksConfig)
	}
	log.V(4).Info("Master authorized networks config update check", "current", existingCluster.MasterAuthorizedNetworksConfig)
	if desiredMasterAuthorizedNetworksConfig != nil {
		log.V(4).Info("Master authorized networks config update check", "desired", desiredMasterAuthorizedNetworksConfig)
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("MasterAuthorizedNetworkConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateAddonsConfig(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredAddonsConfig := infrav1exp.ConvertToSdkAddonsConfig(s.scope.GCPManagedControlPlane.Spec.AddonsConfig)
	if !compareAddonsConfig(desiredAddonsConfig, existingCluster.AddonsConfig) {
		needUpdate = true
		clusterUpdate.DesiredAddonsConfig = desiredAddonsConfig
		log.V(2).Info("AddonsConfig update required", "current", existingCluster.AddonsConfig, "desired", desiredAddonsConfig)
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("AddonsConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateDNSConfig(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredNetworkConfigs := infrav1exp.ConvertToSdkNetworkConfig(s.scope.GCPManagedControlPlane.Spec.NetworkConfig)
	if existingCluster.NetworkConfig != nil {
		if !cmp.Equal(desiredNetworkConfigs.DnsConfig, existingCluster.NetworkConfig.DnsConfig,
			cmpopts.IgnoreUnexported(containerpb.DNSConfig{})) {
			needUpdate = true
			clusterUpdate.DesiredDnsConfig = desiredNetworkConfigs.DnsConfig
			if existingCluster.NetworkConfig == nil {
				log.V(2).Info("Network Configs DnsConfig update required", "current",
					nil, "desired", desiredNetworkConfigs.DnsConfig)
			} else {
				log.V(2).Info("Network Configs DnsConfig update required", "current",
					existingCluster.NetworkConfig.DnsConfig, "desired", desiredNetworkConfigs.DnsConfig)
			}
		}
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("DNSConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateMasterGlobalAccessEnabled(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredPrivateClusterConfig := infrav1exp.ConvertToSdkPrivateClusterConfig(s.scope.GCPManagedControlPlane.Spec.PrivateClusterConfig)
	if !compareMasterGlobalAccessConfig(desiredPrivateClusterConfig.GetMasterGlobalAccessConfig(), existingCluster.PrivateClusterConfig.GetMasterGlobalAccessConfig()) {
		needUpdate = true
		// Only the MasterGlobalAccessConfig is mutable
		clusterUpdate.DesiredPrivateClusterConfig = existingCluster.PrivateClusterConfig
		clusterUpdate.DesiredPrivateClusterConfig.MasterGlobalAccessConfig = desiredPrivateClusterConfig.GetMasterGlobalAccessConfig()
		log.V(2).Info("MasterGlobalAccessConfig update required", "current", existingCluster.PrivateClusterConfig.GetMasterGlobalAccessConfig(),
			"desired", desiredPrivateClusterConfig.GetMasterGlobalAccessConfig())
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("MasterGlobalAccessConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateShieldedNodes(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredShieldedNodes := infrav1exp.ConvertToSdkShieldedNodes(s.scope.GCPManagedControlPlane.Spec.ShieldedNodes)
	if desiredShieldedNodes != nil {
		if existingCluster.ShieldedNodes == nil || existingCluster.ShieldedNodes.Enabled != desiredShieldedNodes.Enabled {
			needUpdate = true
			clusterUpdate.DesiredShieldedNodes = desiredShieldedNodes
			log.V(2).Info("ShieldedNodes update required", "current", existingCluster.ShieldedNodes, "desired", desiredShieldedNodes)
		}
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("ShieldedNodes update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateWorkloadIdentityConfig(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredWorkloadIdentityConfig := infrav1exp.ConvertToSdkWorkloadIdentityConfig(s.scope.GCPManagedControlPlane.Spec.WorkloadIdentityConfig)
	if !compareWorkloadIdentityConfig(desiredWorkloadIdentityConfig, existingCluster.WorkloadIdentityConfig) {
		needUpdate = true
		if desiredWorkloadIdentityConfig == nil {
			clusterUpdate.DesiredWorkloadIdentityConfig = &containerpb.WorkloadIdentityConfig{}
		} else {
			clusterUpdate.DesiredWorkloadIdentityConfig = desiredWorkloadIdentityConfig
		}
		log.V(2).Info("WorkloadIdentityConfig update required", "current", existingCluster.WorkloadIdentityConfig,
			"desired", desiredWorkloadIdentityConfig)
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("WorkloadIdentityConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateLoggingConfig(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{}
	desiredLoggingConfig := infrav1exp.ConvertToSdkLoggingConfig(s.scope.GCPManagedControlPlane.Spec.LoggingConfig)
	if !compareLoggingConfig(desiredLoggingConfig, existingCluster.LoggingConfig) {
		needUpdate = true
		clusterUpdate.DesiredLoggingConfig = desiredLoggingConfig
		log.V(2).Info("LoggingConfig update required", "current", existingCluster.LoggingConfig, "desired", desiredLoggingConfig)
	}

	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name:   s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	log.V(4).Info("LoggingConfig update cluster request. ", "needUpdate", needUpdate, "updateClusterRequest", &updateClusterRequest)
	return needUpdate, &updateClusterRequest
}

func (s *Service) checkDiffAndPrepareUpdateMaintenancePolicy(existingCluster *containerpb.Cluster, log *logr.Logger) (bool, *containerpb.SetMaintenancePolicyRequest, error) {
	needUpdate := false
	setMaintenancePolicyRequest := containerpb.SetMaintenancePolicyRequest{
		Name: s.scope.ClusterFullName(),
	}
	desiredMaintenancePolicy, err := infrav1exp.ConvertToSdkMaintenancePolicy(s.scope.GCPManagedControlPlane.Spec.MaintenancePolicy)
	if err != nil {
		return false, nil, err
	}
	if !compareMaintenancePolicy(desiredMaintenancePolicy, existingCluster.MaintenancePolicy) {
		needUpdate = true
		if desiredMaintenancePolicy != nil {
			desiredMaintenancePolicy.ResourceVersion = existingCluster.MaintenancePolicy.GetResourceVersion()
		} else {
			desiredMaintenancePolicy = &containerpb.MaintenancePolicy{
				ResourceVersion: existingCluster.MaintenancePolicy.GetResourceVersion(),
			}
		}
		setMaintenancePolicyRequest.MaintenancePolicy = desiredMaintenancePolicy
		log.V(2).Info("MaintenancePolicy update required", "current", existingCluster.MaintenancePolicy, "desired", desiredMaintenancePolicy)
	}
	log.V(4).Info("MaintenancePolicy update request. ", "needUpdate", needUpdate, "updateClusterRequest", &setMaintenancePolicyRequest)
	return needUpdate, &setMaintenancePolicyRequest, nil
}
