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

package clusters

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"context"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconcile GKE cluster.
func (s *Service) Reconcile(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling cluster resources")

	s.scope.GCPManagedControlPlane.Status.Ready = true

	cluster, err := s.describeCluster(ctx)
	if err != nil {
		s.scope.GCPManagedControlPlane.Status.Ready = false
		return err
	}
	if cluster == nil {
		log.Info("Cluster not found, creating")
		s.scope.GCPManagedControlPlane.Status.Ready = false
		if err = s.createCluster(ctx); err != nil {
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return err
		}
		log.Info("Cluster provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition)
		return nil
	}

	switch cluster.Status {
	case containerpb.Cluster_PROVISIONING:
		log.Info("Cluster provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition)
		s.scope.GCPManagedControlPlane.Status.Ready = false
		return nil
	case containerpb.Cluster_RECONCILING:
		log.Info("Cluster reconciling in progress")
		return nil
	case containerpb.Cluster_STOPPING:
		log.Info("Cluster stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)
		s.scope.GCPManagedControlPlane.Status.Ready = false
		return nil
	case containerpb.Cluster_ERROR, containerpb.Cluster_DEGRADED:
		var msg string
		if len(cluster.Conditions) > 0 {
			msg = cluster.Conditions[0].GetMessage()
		}
		log.Error(errors.New("Cluster in error/degraded state"), msg, "name", s.scope.GCPManagedControlPlane.Name)
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneErrorReason, clusterv1.ConditionSeverityError, "")
	default:
		break
	}

	needUpdate, updateClusterRequest := s.checkDiffAndPrepareUpdate(*cluster)
	if needUpdate {
		log.Info("Update required")
		err = s.updateCluster(ctx, updateClusterRequest)
		if err != nil {
			return err
		}
		log.Info("Cluster updating in progress")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneUpdatingCondition)
		return nil
	}

	s.scope.SetEndpoint(cluster.Endpoint)
	log.Info("Cluster reconciled")
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition)
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneCreatingCondition, infrav1exp.GKEControlPlaneCreatedReason, clusterv1.ConditionSeverityInfo, "")

	return nil
}

// Delete delete GKE cluster.
func (s *Service) Delete(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Deleting cluster resources")

	cluster, err := s.describeCluster(ctx)
	if err != nil {
		return err
	}
	if cluster == nil {
		log.Info("Cluster already deleted")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition, infrav1exp.GKEControlPlaneDeletedReason, clusterv1.ConditionSeverityInfo, "")
		return nil
	}

	switch cluster.Status {
	case containerpb.Cluster_PROVISIONING:
		log.Info("Cluster provisioning in progress")
		return nil
	case containerpb.Cluster_RECONCILING:
		log.Info("Cluster reconciling in progress")
		return nil
	case containerpb.Cluster_STOPPING:
		log.Info("Cluster stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)
		return nil
	default:
		break
	}

	if err = s.deleteCluster(ctx); err != nil {
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition, infrav1exp.GKEControlPlaneReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return err
	}
	log.Info("Cluster deleting in progress")
	s.scope.GCPManagedControlPlane.Status.Ready = false
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEControlPlaneDeletingReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneDeletingCondition)

	return nil
}

func (s *Service) describeCluster(ctx context.Context) (*containerpb.Cluster, error) {
	log := log.FromContext(ctx)

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
		log.Error(err, "Error getting GKE cluster", "name", s.scope.GCPManagedControlPlane.Name)
		return nil, err
	}

	return cluster, nil
}

func (s *Service) createCluster(ctx context.Context) error {
	log := log.FromContext(ctx)

	cluster := &containerpb.Cluster{
		Name: s.scope.GCPManagedControlPlane.Name,
		Autopilot: &containerpb.Autopilot{
			Enabled: false,
		},
		NodePools: []*containerpb.NodePool{
			{
				Name: "default",
				InitialNodeCount: 1,
			},
		},
		ReleaseChannel: &containerpb.ReleaseChannel{
			Channel: convertReleaseChannel(s.scope.GCPManagedControlPlane.Spec.ReleaseChannel),
		},
	}
	if s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion != nil {
		cluster.InitialClusterVersion = *s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion
	}
	createClusterRequest := &containerpb.CreateClusterRequest{
		Cluster: cluster,
		Parent: s.scope.ClusterLocation(),
	}
	_, err := s.scope.ManagedControlPlaneClient().CreateCluster(ctx, createClusterRequest)
	if err != nil {
		log.Error(err, "Error creating GKE cluster", "name", s.scope.GCPManagedControlPlane.Name)
		return err
	}

	return nil
}

func (s *Service) updateCluster(ctx context.Context, updateClusterRequest containerpb.UpdateClusterRequest) error {
	log := log.FromContext(ctx)

	_, err := s.scope.ManagedControlPlaneClient().UpdateCluster(ctx, &updateClusterRequest)
	if err != nil {
		log.Error(err, "Error updating GKE cluster", "name", s.scope.GCPManagedControlPlane.Name)
		return err
	}

	return nil
}

func (s *Service) deleteCluster(ctx context.Context) error {
	log := log.FromContext(ctx)

	deleteClusterRequest := &containerpb.DeleteClusterRequest{
		Name: s.scope.ClusterFullName(),
	}
	_, err := s.scope.ManagedControlPlaneClient().DeleteCluster(ctx, deleteClusterRequest)
	if err != nil {
		log.Error(err, "Error deleting GKE cluster", "name", s.scope.GCPManagedControlPlane.Name)
		return err
	}

	return nil
}

func convertReleaseChannel(channel *infrav1exp.ReleaseChannel) containerpb.ReleaseChannel_Channel {
	if channel == nil {
		return containerpb.ReleaseChannel_UNSPECIFIED
	}
	switch *channel {
	case infrav1exp.Rapid:
		return containerpb.ReleaseChannel_RAPID
	case infrav1exp.Regular:
		return containerpb.ReleaseChannel_REGULAR
	case infrav1exp.Stable:
		return containerpb.ReleaseChannel_STABLE
	default:
		return containerpb.ReleaseChannel_UNSPECIFIED
	}
}

func (s *Service) checkDiffAndPrepareUpdate(existingCluster containerpb.Cluster) (bool, containerpb.UpdateClusterRequest) {
	needUpdate := false
	clusterUpdate := containerpb.ClusterUpdate{
	}
	// Release channel
	desiredReleaseChannel := convertReleaseChannel(s.scope.GCPManagedControlPlane.Spec.ReleaseChannel)
	if desiredReleaseChannel != existingCluster.ReleaseChannel.Channel {
		needUpdate = true
		clusterUpdate.DesiredReleaseChannel = &containerpb.ReleaseChannel{
			Channel: desiredReleaseChannel,
		}
	}
	// Master version
	if s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion != nil && *s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion != existingCluster.InitialClusterVersion {
		needUpdate = true
		clusterUpdate.DesiredMasterVersion = *s.scope.GCPManagedControlPlane.Spec.ControlPlaneVersion
	}
	updateClusterRequest := containerpb.UpdateClusterRequest{
		Name: s.scope.ClusterFullName(),
		Update: &clusterUpdate,
	}
	return needUpdate, updateClusterRequest
}
