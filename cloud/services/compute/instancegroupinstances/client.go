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

// Package instancegroupinstances provides methods for managing GCP instance groups.
package instancegroupinstances

import (
	"context"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Client wraps GCP SDK.
type Client interface {

	// Instance methods.
	GetInstance(ctx context.Context, project, zone, name string) (*compute.Instance, error)
	// InstanceGroupInstances methods.
	ListInstanceGroupInstances(ctx context.Context, project, zone, name string) (*compute.InstanceGroupManagersListManagedInstancesResponse, error)
	DeleteInstanceGroupInstances(ctx context.Context, project, zone, name string, instances *compute.InstanceGroupManagersDeleteInstancesRequest) (*compute.Operation, error)
	// Disk methods.
	GetDisk(ctx context.Context, project, zone, name string) (*compute.Disk, error)
}

type (
	// GCPClient contains the GCP SDK client.
	GCPClient struct {
		service *compute.Service
	}
)

var _ Client = &GCPClient{}

// NewGCPClient creates a new GCP SDK client.
func NewGCPClient(ctx context.Context, creds []byte) *GCPClient {
	service, err := compute.NewService(ctx, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil
	}
	return &GCPClient{service: service}
}

// GetInstance returns a specific instance in a project and zone.
func (c *GCPClient) GetInstance(_ context.Context, project, zone, name string) (*compute.Instance, error) {
	return c.service.Instances.Get(project, zone, name).Do()
}

// GetDisk returns a specific disk in a project and zone.
func (c *GCPClient) GetDisk(_ context.Context, project, zone, name string) (*compute.Disk, error) {
	return c.service.Disks.Get(project, zone, name).Do()
}

// ListInstanceGroupInstances returns a response that contains the list of managed instances in the instance group.
func (c *GCPClient) ListInstanceGroupInstances(_ context.Context, project, zone, name string) (*compute.InstanceGroupManagersListManagedInstancesResponse, error) {
	return c.service.InstanceGroupManagers.ListManagedInstances(project, zone, name).Do()
}

// DeleteInstanceGroupInstances deletes instances from an instance group in a project and zone.
func (c *GCPClient) DeleteInstanceGroupInstances(_ context.Context, project, zone, name string, instances *compute.InstanceGroupManagersDeleteInstancesRequest) (*compute.Operation, error) {
	return c.service.InstanceGroupManagers.DeleteInstances(project, zone, name, instances).Do()
}
