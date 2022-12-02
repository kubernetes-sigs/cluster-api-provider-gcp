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

package nodepools

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"context"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"reflect"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconcile GKE node pool.
func (s *Service) Reconcile(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling node pool resources")

	s.scope.GCPManagedMachinePool.Status.Ready = true

	nodePool, err := s.describeNodePool(ctx)
	if err != nil {
		s.scope.GCPManagedMachinePool.Status.Ready = false
		return err
	}
	if nodePool == nil {
		log.Info("Node pool not found, creating")
		s.scope.GCPManagedMachinePool.Status.Ready = false
		if err = s.createNodePool(ctx); err != nil {
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolCreatingCondition, infrav1exp.GKEMachinePoolReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return err
		}
		log.Info("Node pool provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolCreatingCondition)
		return nil
	}

	switch nodePool.Status {
	case containerpb.NodePool_PROVISIONING:
		log.Info("Node pool provisioning in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolCreatingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolCreatingCondition)
		s.scope.GCPManagedMachinePool.Status.Ready = false
		return nil
	case containerpb.NodePool_RECONCILING:
		log.Info("Node pool reconciling in progress")
		return nil
	case containerpb.NodePool_STOPPING:
		log.Info("Node pool stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolDeletingCondition)
		s.scope.GCPManagedMachinePool.Status.Ready = false
		return nil
	case containerpb.NodePool_ERROR, containerpb.NodePool_RUNNING_WITH_ERROR:
		var msg string
		if len(nodePool.Conditions) > 0 {
			msg = nodePool.Conditions[0].GetMessage()
		}
		log.Error(errors.New("Node pool in error/degraded state"), msg, "name", s.scope.GCPManagedMachinePool.Name)
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneReadyCondition, infrav1exp.GKEMachinePoolErrorReason, clusterv1.ConditionSeverityError, "")
	default:
		break
	}

	needUpdateVersionOrImage, nodePoolUpdateVersionOrImage := s.checkDiffAndPrepareUpdateVersionOrImage(*nodePool)
	if needUpdateVersionOrImage {
		log.Info("Version/image update required")
		err = s.updateNodePoolVersionOrImage(ctx, nodePoolUpdateVersionOrImage)
		if err != nil {
			return err
		}
		log.Info("Node pool version/image updating in progress")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolUpdatingCondition)
		return nil
	}

	needUpdateSize, setNodePoolSizeRequest := s.checkDiffAndPrepareUpdateSize(*nodePool)
	if needUpdateSize {
		log.Info("Size update required")
		err = s.updateNodePoolSize(ctx, setNodePoolSizeRequest)
		if err != nil {
			return err
		}
		log.Info("Node pool size updating in progress")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolUpdatingCondition)
		return nil
	}

	s.scope.SetReplicas(nodePool.InitialNodeCount)
	log.Info("Node pool reconciled")
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition)
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolCreatingCondition, infrav1exp.GKEMachinePoolCreatedReason, clusterv1.ConditionSeverityInfo, "")

	return nil
}

// Delete delete GKE node pool.
func (s *Service) Delete(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Deleting node pool resources")

	nodePool, err := s.describeNodePool(ctx)
	if err != nil {
		return err
	}
	if nodePool == nil {
		log.Info("Node pool already deleted")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolDeletingCondition, infrav1exp.GKEMachinePoolDeletedReason, clusterv1.ConditionSeverityInfo, "")
		return nil
	}

	switch nodePool.Status {
	case containerpb.NodePool_PROVISIONING:
		log.Info("Node pool provisioning in progress")
		return nil
	case containerpb.NodePool_RECONCILING:
		log.Info("Node pool reconciling in progress")
		return nil
	case containerpb.NodePool_STOPPING:
		log.Info("Node pool stopping in progress")
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolDeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolDeletingCondition)
		return nil
	default:
		break
	}

	if err = s.deleteNodePool(ctx); err != nil {
		conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolDeletingCondition, infrav1exp.GKEMachinePoolReconciliationFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return err
	}
	log.Info("Node pool deleting in progress")
	s.scope.GCPManagedMachinePool.Status.Ready = false
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolReadyCondition, infrav1exp.GKEMachinePoolDeletingReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEMachinePoolDeletingCondition)

	return nil
}

func (s *Service) describeNodePool(ctx context.Context) (*containerpb.NodePool, error) {
	log := log.FromContext(ctx)

	getNodePoolRequest := &containerpb.GetNodePoolRequest{
		Name: s.scope.NodePoolFullName(),
	}
	nodePool, err := s.scope.ManagedMachinePoolClient().GetNodePool(ctx, getNodePoolRequest)
	if err != nil {
		var e *apierror.APIError
		if ok := errors.As(err, &e); ok {
			if e.GRPCStatus().Code() == codes.NotFound {
				return nil, nil
			}
		}
		log.Error(err, "Error getting GKE node pool", "name", s.scope.GCPManagedMachinePool.Name)
		return nil, err
	}

	return nodePool, nil
}

func (s *Service) createNodePool(ctx context.Context) error {
	log := log.FromContext(ctx)

	nodePool := &containerpb.NodePool{
		Name: s.scope.GCPManagedMachinePool.Name,
		InitialNodeCount: s.scope.GCPManagedMachinePool.Spec.NodeCount,
		Config: &containerpb.NodeConfig{
			Labels: s.scope.GCPManagedMachinePool.Spec.KubernetesLabels,
			Taints: infrav1exp.ConvertToSdkTaint(s.scope.GCPManagedMachinePool.Spec.KubernetesTaints),
			Metadata: s.scope.GCPManagedMachinePool.Spec.AdditionalLabels,
		},
	}
	if s.scope.GCPManagedMachinePool.Spec.NodeVersion != nil {
		nodePool.Version = *s.scope.GCPManagedMachinePool.Spec.NodeVersion
	}
	createNodePoolRequest := &containerpb.CreateNodePoolRequest{
		NodePool: nodePool,
		Parent: s.scope.NodePoolLocation(),
	}
	_, err := s.scope.ManagedMachinePoolClient().CreateNodePool(ctx, createNodePoolRequest)
	if err != nil {
		log.Error(err, "Error creating GKE node pool", "name", s.scope.GCPManagedMachinePool.Name)
		return err
	}

	return nil
}

func (s *Service) updateNodePoolVersionOrImage(ctx context.Context, updateNodePoolRequest containerpb.UpdateNodePoolRequest) error {
	log := log.FromContext(ctx)

	_, err := s.scope.ManagedMachinePoolClient().UpdateNodePool(ctx, &updateNodePoolRequest)
	if err != nil {
		log.Error(err, "Error updating GKE node pool image/version", "name", s.scope.GCPManagedMachinePool.Name)
		return err
	}

	return nil
}

func (s *Service) updateNodePoolSize(ctx context.Context, setNodePoolSizeRequest containerpb.SetNodePoolSizeRequest) error {
	log := log.FromContext(ctx)

	_, err := s.scope.ManagedMachinePoolClient().SetNodePoolSize(ctx, &setNodePoolSizeRequest)
	if err != nil {
		log.Error(err, "Error updating GKE node pool size", "name", s.scope.GCPManagedMachinePool.Name)
		return err
	}

	return nil
}

func (s *Service) deleteNodePool(ctx context.Context) error {
	log := log.FromContext(ctx)

	deleteNodePoolRequest := &containerpb.DeleteNodePoolRequest{
		Name: s.scope.NodePoolFullName(),
	}
	_, err := s.scope.ManagedMachinePoolClient().DeleteNodePool(ctx, deleteNodePoolRequest)
	if err != nil {
		log.Error(err, "Error deleting GKE node pool", "name", s.scope.GCPManagedControlPlane.Name)
		return err
	}

	return nil
}

func (s *Service) checkDiffAndPrepareUpdateVersionOrImage(existingNodePool containerpb.NodePool) (bool, containerpb.UpdateNodePoolRequest) {
	needUpdate := false
	updateNodePoolRequest := containerpb.UpdateNodePoolRequest{
		Name: s.scope.NodePoolFullName(),
	}
	// Node version
	if s.scope.GCPManagedMachinePool.Spec.NodeVersion != nil && *s.scope.GCPManagedMachinePool.Spec.NodeVersion != existingNodePool.Version {
		needUpdate = true
		updateNodePoolRequest.NodeVersion = *s.scope.GCPManagedMachinePool.Spec.NodeVersion
	}
	// Kubernetes labels
	desiredKubernetesLabels := map[string]string{}
	if s.scope.GCPManagedMachinePool.Spec.KubernetesLabels != nil {
		desiredKubernetesLabels = s.scope.GCPManagedMachinePool.Spec.KubernetesLabels
	}
	if !reflect.DeepEqual(desiredKubernetesLabels, existingNodePool.Config.Labels) {
		needUpdate = true
		updateNodePoolRequest.Labels = &containerpb.NodeLabels{
			Labels: desiredKubernetesLabels,
		}
	}
	// Kubernetes taints
	desiredKubernetesTaints := infrav1exp.ConvertToSdkTaint(s.scope.GCPManagedMachinePool.Spec.KubernetesTaints)
	if !reflect.DeepEqual(desiredKubernetesTaints, existingNodePool.Config.Taints) {
		needUpdate = true
		updateNodePoolRequest.Taints = &containerpb.NodeTaints{
			Taints: desiredKubernetesTaints,
		}
	}
	return needUpdate, updateNodePoolRequest
}

func (s *Service) checkDiffAndPrepareUpdateSize(existingNodePool containerpb.NodePool) (bool, containerpb.SetNodePoolSizeRequest) {
	needUpdate := false
	setNodePoolSizeRequest := containerpb.SetNodePoolSizeRequest{
		Name: s.scope.NodePoolFullName(),
	}
	if s.scope.GCPManagedMachinePool.Spec.NodeCount != existingNodePool.InitialNodeCount {
		needUpdate = true
		setNodePoolSizeRequest.NodeCount = s.scope.GCPManagedMachinePool.Spec.NodeCount
	}
	return needUpdate, setNodePoolSizeRequest
}
