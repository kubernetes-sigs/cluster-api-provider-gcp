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

package scope

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/option"

	"sigs.k8s.io/cluster-api/util/conditions"

	container "cloud.google.com/go/container/apiv1"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/pkg/errors"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// APIServerPort is the port of the GKE api server.
	APIServerPort = 443
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ManagedControlPlaneScopeParams struct {
	CredentialsClient      *credentials.IamCredentialsClient
	ManagedClusterClient   *container.ClusterManagerClient
	Client                 client.Client
	Cluster                *clusterv1.Cluster
	GCPManagedCluster      *infrav1exp.GCPManagedCluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(ctx context.Context, params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GCPManagedCluster == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedCluster")
	}
	if params.GCPManagedControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedControlPlane")
	}

	var credentialData []byte
	var credential *Credential
	var err error
	if params.GCPManagedCluster.Spec.CredentialsRef != nil {
		credentialData, err = getCredentialDataFromRef(ctx, params.GCPManagedCluster.Spec.CredentialsRef, params.Client)
	} else {
		credentialData, err = getCredentialDataFromMount()
	}
	if err != nil {
		return nil, errors.Errorf("failed to get credential data: %v", err)
	}

	credential, err = parseCredential(credentialData)
	if err != nil {
		return nil, errors.Errorf("failed to parse credential data: %v", err)
	}

	if params.ManagedClusterClient == nil {
		var managedClusterClient *container.ClusterManagerClient
		managedClusterClient, err = container.NewClusterManagerClient(ctx, option.WithCredentialsJSON(credentialData))
		if err != nil {
			return nil, errors.Errorf("failed to create gcp managed cluster client: %v", err)
		}
		params.ManagedClusterClient = managedClusterClient
	}
	if params.CredentialsClient == nil {
		var credentialsClient *credentials.IamCredentialsClient
		credentialsClient, err = credentials.NewIamCredentialsClient(ctx, option.WithCredentialsJSON(credentialData))
		if err != nil {
			return nil, errors.Errorf("failed to create gcp credentials client: %v", err)
		}
		params.CredentialsClient = credentialsClient
	}

	helper, err := patch.NewHelper(params.GCPManagedControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		client:                 params.Client,
		Cluster:                params.Cluster,
		GCPManagedCluster:      params.GCPManagedCluster,
		GCPManagedControlPlane: params.GCPManagedControlPlane,
		mcClient:               params.ManagedClusterClient,
		credentialsClient:      params.CredentialsClient,
		credential:             credential,
		patchHelper:            helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster                *clusterv1.Cluster
	GCPManagedCluster      *infrav1exp.GCPManagedCluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	mcClient               *container.ClusterManagerClient
	credentialsClient      *credentials.IamCredentialsClient
	credential             *Credential
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
	s.credentialsClient.Close()
	return s.PatchObject()
}

// ConditionSetter return a condition setter (which is GCPManagedControlPlane itself).
func (s *ManagedControlPlaneScope) ConditionSetter() conditions.Setter {
	return s.GCPManagedControlPlane
}

// Client returns a k8s client.
func (s *ManagedControlPlaneScope) Client() client.Client {
	return s.client
}

// ManagedControlPlaneClient returns a client used to interact with GKE.
func (s *ManagedControlPlaneScope) ManagedControlPlaneClient() *container.ClusterManagerClient {
	return s.mcClient
}

// CredentialsClient returns a client used to interact with IAM.
func (s *ManagedControlPlaneScope) CredentialsClient() *credentials.IamCredentialsClient {
	return s.credentialsClient
}

// GetCredential returns the credential data.
func (s *ManagedControlPlaneScope) GetCredential() *Credential {
	return s.credential
}

func parseLocation(location string) (region string, zone *string) {
	parts := strings.Split(location, "-")
	region = strings.Join(parts[:2], "-")
	if len(parts) == 3 {
		return region, &parts[2]
	}
	return region, nil
}

// Region returns the region of the GKE cluster.
func (s *ManagedControlPlaneScope) Region() string {
	region, _ := parseLocation(s.GCPManagedControlPlane.Spec.Location)
	return region
}

// ClusterLocation returns the location of the cluster.
func (s *ManagedControlPlaneScope) ClusterLocation() string {
	return fmt.Sprintf("projects/%s/locations/%s", s.GCPManagedControlPlane.Spec.Project, s.Region())
}

// ClusterFullName returns the full name of the cluster.
func (s *ManagedControlPlaneScope) ClusterFullName() string {
	return fmt.Sprintf("%s/clusters/%s", s.ClusterLocation(), s.GCPManagedControlPlane.Name)
}

// SetEndpoint sets the Endpoint of GCPManagedControlPlane.
func (s *ManagedControlPlaneScope) SetEndpoint(host string) {
	s.GCPManagedControlPlane.Spec.Endpoint = clusterv1.APIEndpoint{
		Host: host,
		Port: APIServerPort,
	}
}
