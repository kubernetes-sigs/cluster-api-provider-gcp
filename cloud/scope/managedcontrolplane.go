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
	"context"
	"fmt"
	"sigs.k8s.io/cluster-api/util/conditions"
	"strings"

	"cloud.google.com/go/container/apiv1"
	"github.com/pkg/errors"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ManagedControlPlaneScopeParams struct {
	ManagedClusterClient *container.ClusterManagerClient
	Client     client.Client
	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	//if params.Cluster == nil {
	//	return nil, errors.New("failed to generate new scope from nil Cluster")
	//}
	if params.GCPManagedControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedControlPlane")
	}

	if params.ManagedClusterClient == nil {
		managedClusterClient, err := container.NewClusterManagerClient(context.TODO())
		if err != nil {
			return nil, errors.Errorf("failed to create gcp managed cluster client: %v", err)
		}

		params.ManagedClusterClient = managedClusterClient
	}

	helper, err := patch.NewHelper(params.GCPManagedControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		GCPManagedControlPlane:  params.GCPManagedControlPlane,
		mcClient: params.ManagedClusterClient,
		patchHelper: helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	mcClient *container.ClusterManagerClient

	AllNodePools []infrav1exp.GCPManagedMachinePool
}

// PatchObject persists the managed control plane configuration and status.
func (s *ManagedControlPlaneScope) PatchObject() error {
	return s.patchHelper.Patch(
		context.TODO(),
		s.GCPManagedControlPlane,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			infrav1exp.GKEControlPlaneReadyCondition,
			infrav1exp.GKEControlPlaneCreatingCondition,
			infrav1exp.GKEControlPlaneUpdatingCondition,
			infrav1exp.GKEControlPlaneDeletingCondition,
		}})
}

// Close closes the current scope persisting the managed control plane configuration and status.
func (s *ManagedControlPlaneScope) Close() error {
	s.mcClient.Close()
	return s.PatchObject()
}

func (s *ManagedControlPlaneScope) ConditionSetter() conditions.Setter {
	return s.GCPManagedControlPlane
}

func (s *ManagedControlPlaneScope) ManagedControlPlaneClient() *container.ClusterManagerClient {
	return s.mcClient
}

func (s *ManagedControlPlaneScope) GetAllNodePools() ([]infrav1exp.GCPManagedMachinePool, error) {
	if s.AllNodePools == nil {
		opt1 := client.InNamespace(s.GCPManagedControlPlane.Namespace)
		opt2 := client.MatchingLabels(map[string]string{
			clusterv1.ClusterLabelName: s.Cluster.Name,
		})

		machinePoolList := &infrav1exp.GCPManagedMachinePoolList{}
		if err := s.client.List(context.TODO(), machinePoolList, opt1, opt2); err != nil {
			return nil, err
		}
		s.AllNodePools = machinePoolList.Items
	}

	return s.AllNodePools, nil
}

func parseLocation(location string) (region string, zone *string) {
	parts := strings.Split(location, "-")
	region = strings.Join(parts[:2], "-")
	if len(parts) == 3 {
		return region, &parts[2]
	} else {
		return region, nil
	}
}

func (s *ManagedControlPlaneScope) Region() string {
	region, _ := parseLocation(s.GCPManagedControlPlane.Spec.Location)
	return region
}

func (s *ManagedControlPlaneScope) ClusterLocation() string {
	return fmt.Sprintf("projects/%s/locations/%s", s.GCPManagedControlPlane.Spec.Project, s.Region())
}

func (s *ManagedControlPlaneScope) ClusterFullName() string {
	return fmt.Sprintf("%s/clusters/%s", s.ClusterLocation(), s.GCPManagedControlPlane.Name)
}

func (s *ManagedControlPlaneScope) SetEndpoint(host string) {
	s.GCPManagedControlPlane.Spec.Endpoint = clusterv1.APIEndpoint{
		Host: host,
		Port: 443,
	}
}
