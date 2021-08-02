/*
Copyright 2018 The Kubernetes Authors.

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
	"google.golang.org/api/container/v1"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type GKEClusterScopeParams struct {
	GKEClients
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	GKECluster *infrav1.GKECluster
}

// NewGKEClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewGKEClusterScope(params GKEClusterScopeParams) (*GKEClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GKECluster == nil {
		return nil, errors.New("failed to generate new scope from nil GCPCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	containerSvc, err := container.NewService(context.TODO())
	if err != nil {
		return nil, errors.Errorf("failed to create gcp compute client: %v", err)
	}

	if params.GKEClients.Container == nil {
		params.GKEClients.Container = containerSvc
	}

	helper, err := patch.NewHelper(params.GKECluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &GKEClusterScope{
		Logger:      params.Logger,
		client:      params.Client,
		GKEClients:  params.GKEClients,
		Cluster:     params.Cluster,
		GKECluster:  params.GKECluster,
		patchHelper: helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type GKEClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	GKEClients
	Cluster    *clusterv1.Cluster
	GKECluster *infrav1.GKECluster
}

// Project returns the current project name.
func (s *GKEClusterScope) Project() string {
	return s.GKECluster.Spec.Project
}

// Name returns the cluster name.
func (s *GKEClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *GKEClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Region returns the cluster region.
func (s *GKEClusterScope) Region() string {
	return s.GKECluster.Spec.Region
}

// PatchObject persists the cluster configuration and status.
func (s *GKEClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.GKECluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *GKEClusterScope) Close() error {
	return s.PatchObject()
}
