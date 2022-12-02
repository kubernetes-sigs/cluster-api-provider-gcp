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

package scope

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"context"
	"fmt"
	"sigs.k8s.io/cluster-api/util/conditions"

	"cloud.google.com/go/container/apiv1"
	"github.com/pkg/errors"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedMachinePoolScopeParams defines the input parameters used to create a new Scope.
type ManagedMachinePoolScopeParams struct {
	ManagedClusterClient *container.ClusterManagerClient
	Client     client.Client
	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	GCPManagedMachinePool *infrav1exp.GCPManagedMachinePool
}

// NewManagedMachinePoolScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedMachinePoolScope(params ManagedMachinePoolScopeParams) (*ManagedMachinePoolScope, error) {
	//if params.Cluster == nil {
	//	return nil, errors.New("failed to generate new scope from nil Cluster")
	//}
	if params.GCPManagedControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedControlPlane")
	}
	if params.GCPManagedMachinePool == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedMachinePool")
	}

	if params.ManagedClusterClient == nil {
		managedClusterClient, err := container.NewClusterManagerClient(context.TODO())
		if err != nil {
			return nil, errors.Errorf("failed to create gcp managed cluster client: %v", err)
		}

		params.ManagedClusterClient = managedClusterClient
	}

	helper, err := patch.NewHelper(params.GCPManagedMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedMachinePoolScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		GCPManagedControlPlane: params.GCPManagedControlPlane,
		GCPManagedMachinePool:  params.GCPManagedMachinePool,
		mcClient: params.ManagedClusterClient,
		patchHelper: helper,
	}, nil
}

// ManagedMachinePoolScope defines the basic context for an actuator to operate upon.
type ManagedMachinePoolScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	GCPManagedMachinePool *infrav1exp.GCPManagedMachinePool
	mcClient *container.ClusterManagerClient
}

// PatchObject persists the managed control plane configuration and status.
func (s *ManagedMachinePoolScope) PatchObject() error {
	return s.patchHelper.Patch(
		context.TODO(),
		s.GCPManagedMachinePool,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			infrav1exp.GKEMachinePoolReadyCondition,
			infrav1exp.GKEMachinePoolCreatingCondition,
			infrav1exp.GKEMachinePoolUpdatingCondition,
			infrav1exp.GKEMachinePoolDeletingCondition,
		}})
}

// Close closes the current scope persisting the managed control plane configuration and status.
func (s *ManagedMachinePoolScope) Close() error {
	s.mcClient.Close()
	return s.PatchObject()
}

func (s *ManagedMachinePoolScope) ConditionSetter() conditions.Setter {
	return s.GCPManagedMachinePool
}

func (s *ManagedMachinePoolScope) ManagedMachinePoolClient() *container.ClusterManagerClient {
	return s.mcClient
}

func ConvertToSdkNodePool(nodePool infrav1exp.GCPManagedMachinePool) *containerpb.NodePool {
	nodePoolName := nodePool.Spec.NodePoolName
	if len(nodePoolName) == 0 {
		nodePoolName = nodePool.Name
	}
	sdkNodePool := containerpb.NodePool{
		Name: nodePoolName,
		InitialNodeCount: nodePool.Spec.NodeCount,
		Config: &containerpb.NodeConfig{
			Labels: nodePool.Spec.KubernetesLabels,
			Taints: infrav1exp.ConvertToSdkTaint(nodePool.Spec.KubernetesTaints),
			Metadata: nodePool.Spec.AdditionalLabels,
		},
	}
	if nodePool.Spec.NodeVersion != nil {
		sdkNodePool.Version = *nodePool.Spec.NodeVersion
	}
	return &sdkNodePool
}

func ConvertToSdkNodePools(nodePools []infrav1exp.GCPManagedMachinePool) []*containerpb.NodePool {
	res := []*containerpb.NodePool{}
	for _, nodePool := range nodePools {
		res = append(res, ConvertToSdkNodePool(nodePool))
	}
	return res
}

func (s *ManagedMachinePoolScope) SetReplicas(replicas int32) {
	s.GCPManagedMachinePool.Status.Replicas = replicas
}

func (s *ManagedMachinePoolScope) NodePoolName() string {
	if len(s.GCPManagedMachinePool.Spec.NodePoolName) > 0 {
		return s.GCPManagedMachinePool.Spec.NodePoolName
	} else {
		return s.GCPManagedMachinePool.Name
	}
}

func (s *ManagedMachinePoolScope) Region() string {
	region, _ := parseLocation(s.GCPManagedControlPlane.Spec.Location)
	return region
}

func (s *ManagedMachinePoolScope) NodePoolLocation() string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s", s.GCPManagedControlPlane.Spec.Project, s.Region(), s.GCPManagedControlPlane.Name)
}

func (s *ManagedMachinePoolScope) NodePoolFullName() string {
	return fmt.Sprintf("%s/nodePools/%s", s.NodePoolLocation(), s.NodePoolName())
}
