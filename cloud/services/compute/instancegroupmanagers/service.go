/*
Copyright 2022 The Kubernetes Authors.

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

package instancegroupmanagers

import (
	"context"

	k8scloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"google.golang.org/api/compute/v1"
	computega "google.golang.org/api/compute/v1"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
)

type instanceGroupManagersClient interface {
	Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*computega.InstanceGroupManager, error)
	// List(ctx context.Context, zone string, fl *filter.F, options ...Option) ([]*computega.InstanceGroupManager, error)
	Insert(ctx context.Context, key *meta.Key, obj *computega.InstanceGroupManager, options ...k8scloud.Option) error
	Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
	// CreateInstances(context.Context, *meta.Key, *computega.InstanceGroupManagersCreateInstancesRequest, ...Option) error
	// DeleteInstances(context.Context, *meta.Key, *computega.InstanceGroupManagersDeleteInstancesRequest, ...Option) error
	Resize(context.Context, *meta.Key, int64, ...k8scloud.Option) error
	SetInstanceTemplate(context.Context, *meta.Key, *computega.InstanceGroupManagersSetInstanceTemplateRequest, ...k8scloud.Option) error
}

// Scope is an interfaces that hold used methods.
type Scope interface {
	// cloud.Cluster
	// SubnetSpecs() []*compute.Subnetwork

	// Zone() string

	Cloud() cloud.Cloud

	// GetBootstrapData(ctx context.Context) (string, error)

	// Zone() string

	// // InstanceSpec builds a compute.Instance spec for a machine in the specified zone.
	// // We reuse this (somewhat complex) logic when building an instanceTemplate
	// InstanceSpec(ctx context.Context, zone string) *compute.Instance

	// Replicas() *int32

	// Zones() []string
	// Region() string

	// // InstanceGroupManagerName is the name to use for the instanceGroupManager GCP resource
	// InstanceGroupManagerName() string

	// // BaseInstanceName is the name to use for the base instance
	// BaseInstanceName() string

	// InstanceGroupManagerResource returns the desired instanceGroupManager
	InstanceGroupManagerResource(instanceTemplateKey *meta.Key) (*compute.InstanceGroupManager, error)

	// InstanceGroupManagerResourceName returns the instanceGroupManager selfLink
	InstanceGroupManagerResourceName() (*meta.Key, error)
}

// Service implements managed instance groups reconciler.
type Service struct {
	scope                 Scope
	instanceGroupManagers instanceGroupManagersClient
}

// var _ cloud.Reconciler = &Service{}

// New returns Service from given scope.
func New(scope Scope) *Service {
	cloudScope := scope.Cloud()
	// if scope.IsSharedVpc() {
	// 	cloudScope = scope.NetworkCloud()
	// }

	return &Service{
		scope:                 scope,
		instanceGroupManagers: cloudScope.InstanceGroupManagers(),
	}
}
