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

package instancetemplates

import (
	"context"

	k8scloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computega "google.golang.org/api/compute/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
)

type instancetemplatesInterface interface {
	Get(ctx context.Context, key *meta.Key, options ...k8scloud.Option) (*computega.InstanceTemplate, error)
	List(ctx context.Context, fl *filter.F, options ...k8scloud.Option) ([]*computega.InstanceTemplate, error)
	Insert(ctx context.Context, key *meta.Key, obj *computega.InstanceTemplate, options ...k8scloud.Option) error
	Delete(ctx context.Context, key *meta.Key, options ...k8scloud.Option) error
	// CreateInstances(context.Context, *meta.Key, *computega.InstanceGroupManagersCreateInstancesRequest, ...Option) error
	// DeleteInstances(context.Context, *meta.Key, *computega.InstanceGroupManagersDeleteInstancesRequest, ...Option) error
	// Resize(context.Context, *meta.Key, int64, ...Option) error
	// SetInstanceTemplate(context.Context, *meta.Key, *computega.InstanceGroupManagersSetInstanceTemplateRequest, ...Option) error
}

// Scope is an interfaces that hold used methods.
type Scope interface {
	// cloud.Cluster
	// SubnetSpecs() []*compute.Subnetwork
	Cloud() cloud.Cloud

	// GetBootstrapData(ctx context.Context) (string, error)

	// Zone() string

	// InstanceSpec(ctx context.Context, zone string) *compute.Instance

	// InstanceTemplateResource returns the desired instanceTemplate
	InstanceTemplateResource(ctx context.Context) (*computega.InstanceTemplate, error)

	// BaseInstanceTemplateResourceName returns the base instanceTemplate selfLink
	BaseInstanceTemplateResourceName() (*meta.Key, error)
}

// Service implements managed instance groups reconciler.
type Service struct {
	scope             Scope
	instanceTemplates instancetemplatesInterface
}

// var _ cloud.Reconciler = &Service{}

// New returns Service from given scope.
func New(scope Scope) *Service {
	cloudScope := scope.Cloud()

	return &Service{
		scope:             scope,
		instanceTemplates: cloudScope.InstanceTemplates(),
	}
}
