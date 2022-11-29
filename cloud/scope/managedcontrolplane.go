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

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ManagedControlPlaneScopeParams struct {
	GCPServices
	Client     client.Client
	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GCPManagedControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedControlPlane")
	}

	if params.GCPServices.Compute == nil {
		computeSvc, err := compute.NewService(context.TODO())
		if err != nil {
			return nil, errors.Errorf("failed to create gcp compute client: %v", err)
		}

		params.GCPServices.Compute = computeSvc
	}

	helper, err := patch.NewHelper(params.GCPManagedControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		GCPManagedControlPlane:  params.GCPManagedControlPlane,
		GCPServices: params.GCPServices,
		patchHelper: helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	GCPServices
}

// PatchObject persists the managed control plane configuration and status.
func (s *ManagedControlPlaneScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.GCPManagedControlPlane)
}

// Close closes the current scope persisting the managed control plane configuration and status.
func (s *ManagedControlPlaneScope) Close() error {
	return s.PatchObject()
}
