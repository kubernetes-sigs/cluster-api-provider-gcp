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

// Package instancegroupinstances provides methods for managing GCP instance group instances.
package instancegroupinstances

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type (
	// Service is a service for managing GCP instance group instances.
	Service struct {
		scope *scope.MachinePoolMachineScope
		Client
	}
)

var _ cloud.ReconcilerWithResult = &Service{}

// New creates a new instance group service.
func New(scope *scope.MachinePoolMachineScope) *Service {
	creds, err := scope.GetGCPClientCredentials()
	if err != nil {
		return nil
	}

	return &Service{
		scope:  scope,
		Client: NewGCPClient(context.Background(), creds),
	}
}

// Reconcile gets/creates/updates a instance group.
func (s *Service) Reconcile(ctx context.Context) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling Instance Group Instances")

	// Fetch the instance.
	instance, err := s.GetInstance(ctx, s.scope.Project(), s.scope.Zone(), s.scope.Name())
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch the instances disk.
	disk, err := s.GetDisk(ctx, s.scope.Project(), s.scope.Zone(), s.scope.Name())
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the GCPMachinePoolMachine status.
	s.scope.GCPMachinePoolMachine.Status.InstanceName = instance.Name

	// Update Node status with the instance information. If the node is not found, requeue.
	if nodeFound, err := s.scope.UpdateNodeStatus(ctx); err != nil {
		log.Error(err, "Failed to update Node status")
		return ctrl.Result{}, err
	} else if !nodeFound {
		log.Info("Node not found, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Update hasLatestModelApplied status.
	latestModel, err := s.scope.HasLatestModelApplied(ctx, disk)
	if err != nil {
		log.Error(err, "Failed to check if the latest model is applied")
		return ctrl.Result{}, err
	}

	// Update the GCPMachinePoolMachine status.
	s.scope.GCPMachinePoolMachine.Status.LatestModelApplied = latestModel
	s.scope.SetReady()

	return ctrl.Result{}, nil
}

// Delete deletes the instance group.
func (s *Service) Delete(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Deleting Instance Group Instances")

	if s.scope.GCPMachinePoolMachine.Status.ProvisioningState != v1beta1.Deleting {
		log.Info("Deleting instance", "instance", s.scope.Name())
		// Cordon and drain the node before deleting the instance.
		if err := s.scope.CordonAndDrainNode(ctx); err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}

		// Delete the instance group instance
		_, err := s.DeleteInstanceGroupInstances(ctx, s.scope.Project(), s.scope.Zone(), s.scope.GCPMachinePool.Name, &compute.InstanceGroupManagersDeleteInstancesRequest{
			Instances: []string{fmt.Sprintf("zones/%s/instances/%s", s.scope.Zone(), s.scope.Name())},
		})
		if err != nil {
			log.Info("Assuming the instance is already deleted", "error", gcperrors.PrintGCPError(err))
			return ctrl.Result{}, nil
		}

		// Update the GCPMachinePoolMachine status.
		s.scope.GCPMachinePoolMachine.Status.ProvisioningState = v1beta1.Deleting

		// Wait for the instance to be deleted before proceeding.
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Waiting for instance to be deleted", "instance", s.scope.Name())
	// List the instance group instances to check if the instance is deleted.
	instances, err := s.ListInstanceGroupInstances(ctx, s.scope.Project(), s.scope.Zone(), s.scope.GCPMachinePool.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, instance := range instances.ManagedInstances {
		if instance.Name == s.scope.Name() {
			log.Info("Instance is still deleting")
			return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}
