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

// Package instancegroups provides methods for managing GCP instance groups.
package instancegroups

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Client wraps GCP SDK.
type Client interface {
	// InstanceGroup Interfaces
	GetInstanceGroup(ctx context.Context, project, zone, name string) (*compute.InstanceGroupManager, error)
	CreateInstanceGroup(ctx context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error)
	UpdateInstanceGroup(ctx context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error)
	SetInstanceGroupTemplate(ctx context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error)
	DeleteInstanceGroup(ctx context.Context, project, zone, name string) (*compute.Operation, error)
	// InstanceGroupTemplate Interfaces
	GetInstanceTemplate(ctx context.Context, project, name string) (*compute.InstanceTemplate, error)
	CreateInstanceTemplate(ctx context.Context, project string, instanceTemplate *compute.InstanceTemplate) (*compute.Operation, error)
	DeleteInstanceTemplate(ctx context.Context, project, name string) (*compute.Operation, error)
	WaitUntilOperationCompleted(project, operation string) error
	WaitUntilComputeOperationCompleted(project, zone, operation string) error
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

// GetInstanceGroup returns a specific instance group in a project and zone.
func (c *GCPClient) GetInstanceGroup(_ context.Context, project, zone, name string) (*compute.InstanceGroupManager, error) {
	return c.service.InstanceGroupManagers.Get(project, zone, name).Do()
}

// CreateInstanceGroup creates a new instance group in a project and zone.
func (c *GCPClient) CreateInstanceGroup(_ context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error) {
	return c.service.InstanceGroupManagers.Insert(project, zone, instanceGroup).Do()
}

// UpdateInstanceGroup updates an instance group in a project and zone.
func (c *GCPClient) UpdateInstanceGroup(_ context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error) {
	return c.service.InstanceGroupManagers.Patch(project, zone, instanceGroup.Name, instanceGroup).Do()
}

// SetInstanceGroupTemplate sets an instance group template in a project and zone.
func (c *GCPClient) SetInstanceGroupTemplate(_ context.Context, project, zone string, instanceGroup *compute.InstanceGroupManager) (*compute.Operation, error) {
	return c.service.InstanceGroupManagers.SetInstanceTemplate(project, zone, instanceGroup.Name, &compute.InstanceGroupManagersSetInstanceTemplateRequest{
		InstanceTemplate: instanceGroup.InstanceTemplate,
	}).Do()
}

// DeleteInstanceGroup deletes an instance group in a project and zone.
func (c *GCPClient) DeleteInstanceGroup(_ context.Context, project, zone, name string) (*compute.Operation, error) {
	return c.service.InstanceGroupManagers.Delete(project, zone, name).Do()
}

// GetInstanceTemplate returns a specific instance template in a project.
func (c *GCPClient) GetInstanceTemplate(_ context.Context, project, name string) (*compute.InstanceTemplate, error) {
	return c.service.InstanceTemplates.Get(project, name).Do()
}

// CreateInstanceTemplate creates a new instance template in a project.
func (c *GCPClient) CreateInstanceTemplate(_ context.Context, project string, instanceTemplate *compute.InstanceTemplate) (*compute.Operation, error) {
	return c.service.InstanceTemplates.Insert(project, instanceTemplate).Do()
}

// DeleteInstanceTemplate deletes an instance template in a project.
func (c *GCPClient) DeleteInstanceTemplate(_ context.Context, project, name string) (*compute.Operation, error) {
	return c.service.InstanceTemplates.Delete(project, name).Do()
}

// WaitUntilOperationCompleted waits for an operation to complete.
func (c *GCPClient) WaitUntilOperationCompleted(projectID, operationName string) error {
	for {
		operation, err := c.service.GlobalOperations.Get(projectID, operationName).Do()
		if err != nil {
			return err
		}
		if operation.Status == "DONE" {
			if operation.Error != nil {
				return fmt.Errorf("operation failed: %v", operation.Error.Errors)
			}
			return nil
		}
		// Wait 1s before checking again to avoid spamming the API.
		time.Sleep(1 * time.Second)
	}
}

// WaitUntilComputeOperationCompleted waits for a compute operation to complete.
func (c *GCPClient) WaitUntilComputeOperationCompleted(project, zone, operationName string) error {
	for {
		operation, err := c.service.ZoneOperations.Get(project, zone, operationName).Do()
		if err != nil {
			return err
		}
		if operation.Status == "DONE" {
			if operation.Error != nil {
				return fmt.Errorf("operation failed: %v", operation.Error.Errors)
			}
			return nil
		}
		// Wait 1s before checking again to avoid spamming the API.
		time.Sleep(1 * time.Second)
	}
}
