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
	"strings"
	"time"

	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type (
	// Service is a service for managing GCP instance groups.
	Service struct {
		scope *scope.MachinePoolScope
		Client
	}
)

var _ cloud.ReconcilerWithResult = &Service{}

// New creates a new instance group service.
func New(scope *scope.MachinePoolScope) *Service {
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
	log.Info("Reconciling Instance Group")

	// Get the bootstrap data.
	bootStrapToken, err := s.scope.GetBootstrapData()
	if err != nil {
		return ctrl.Result{}, err
	}
	// If the bootstrap data is empty, requeue. This is needed because the bootstrap data is not available until the bootstrap token is created.
	if bootStrapToken == "" {
		log.Info("Bootstrap token is empty, requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Build the instance template based on the GCPMachinePool Spec and the bootstrap data.
	instanceTemplate := s.scope.InstanceGroupTemplateBuilder(bootStrapToken)
	instanceTemplateHash, err := s.scope.GetInstanceTemplateHash(instanceTemplate)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the instance template name.
	instanceTemplateName := s.scope.GCPMachinePool.Name + "-" + instanceTemplateHash

	// Get the instance template if it exists. If it doesn't, create it. If it does, update it.
	_, err = s.Client.GetInstanceTemplate(ctx, s.scope.Project(), instanceTemplateName)
	switch {
	case err != nil && !gcperrors.IsNotFound(err):
		log.Error(err, "Error looking for instance template")
		return ctrl.Result{}, err
	case err != nil && gcperrors.IsNotFound(err):
		log.Info("Instance template not found, creating")
		err = s.createInstanceTemplate(ctx, instanceTemplateName, instanceTemplate)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	instanceGroup, err := s.Client.GetInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
	var patched bool
	switch {
	case err != nil && !gcperrors.IsNotFound(err):
		log.Error(err, "Error looking for instance group")
		return ctrl.Result{}, err
	case err != nil && gcperrors.IsNotFound(err):
		log.Info("Instance group not found, creating")
		err = s.createInstanceGroup(ctx, instanceTemplateName)
		if err != nil {
			return ctrl.Result{}, err
		}
	case err == nil:
		log.Info("Instance group found", "instance group", instanceGroup.Name)
		patched, err = s.patchInstanceGroup(ctx, instanceTemplateName, instanceGroup)
		if err != nil {
			log.Error(err, "Error updating instance group")
			return ctrl.Result{}, err
		}
		err = s.removeOldInstanceTemplate(ctx, instanceTemplateName)
		if err != nil {
			log.Error(err, "Error removing old instance templates")
			return ctrl.Result{}, err
		}
	}

	// Get the instance group again if it was patched. This is needed to get the updated state. If it wasn't patched, use the instance group from the previous step.
	if patched {
		log.Info("Instance group patched, getting updated instance group")
		instanceGroup, err = s.Client.GetInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
		if err != nil {
			log.Error(err, "Error getting instance group")
			return ctrl.Result{}, err
		}
	}
	// List the instance group instances. This is needed to get the provider IDs.
	instanceGroupInstances, err := s.Client.ListInstanceGroupInstances(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
	if err != nil {
		log.Error(err, "Error listing instance group instances")
		return ctrl.Result{}, err
	}

	// Set the MIG state and instances. This is needed to set the status.
	if instanceGroup != nil && instanceGroupInstances != nil {
		s.scope.SetMIGState(instanceGroup)
		s.scope.SetMIGInstances(instanceGroupInstances.ManagedInstances)
	} else {
		err = fmt.Errorf("instance group or instance group list is nil")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// createInstanceTemplate creates the instance template.
func (s *Service) createInstanceTemplate(ctx context.Context, instanceTemplateName string, instanceTemplate *compute.InstanceTemplate) error {
	// Set the instance template name. This is used to identify the instance template.
	instanceTemplate.Name = instanceTemplateName

	// Create the instance template in GCP.
	instanceTemplateCreateOperation, err := s.Client.CreateInstanceTemplate(ctx, s.scope.Project(), instanceTemplate)
	if err != nil {
		return err
	}

	// Wait for the instance group to be deleted
	err = s.WaitUntilOperationCompleted(s.scope.Project(), instanceTemplateCreateOperation.Name)
	if err != nil {
		return err
	}

	return nil
}

// createInstanceGroup creates the instance group.
func (s *Service) createInstanceGroup(ctx context.Context, instanceTemplateName string) error {
	// Create the instance group in GCP.
	igCreationOperation, err := s.Client.CreateInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.InstanceGroupBuilder(instanceTemplateName))
	if err != nil {
		return err
	}

	// Wait for the instance group to be deleted
	err = s.WaitUntilComputeOperationCompleted(s.scope.Project(), s.scope.Zone(), igCreationOperation.Name)
	if err != nil {
		return err
	}

	return nil
}

// patchInstanceGroup patches the instance group.
func (s *Service) patchInstanceGroup(ctx context.Context, instanceTemplateName string, instanceGroup *compute.InstanceGroupManager) (bool, error) {
	log := log.FromContext(ctx)

	// Reconcile replicas.
	err := s.scope.ReconcileReplicas(ctx, instanceGroup)
	if err != nil {
		log.Error(err, "Error reconciling replicas")
		return false, err
	}

	lastSlashTemplateURI := strings.LastIndex(instanceGroup.InstanceTemplate, "/")
	fetchedInstanceTemplateName := instanceGroup.InstanceTemplate[lastSlashTemplateURI+1:]

	patched := false
	// Check if instance group is already using the instance template.
	if fetchedInstanceTemplateName != instanceTemplateName {
		log.Info("Instance group is not using the latest instance template, setting instance template", "instance group", instanceGroup.InstanceTemplate, "instance template", instanceTemplateName)
		// Set instance template.
		setInstanceTemplateOperation, err := s.Client.SetInstanceGroupTemplate(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.InstanceGroupUpdate(instanceTemplateName))
		if err != nil {
			log.Error(err, "Error setting instance group template")
			return false, err
		}

		err = s.WaitUntilComputeOperationCompleted(s.scope.Project(), s.scope.Zone(), setInstanceTemplateOperation.Name)
		if err != nil {
			log.Error(err, "Error waiting for instance group template operation to complete")
			return false, err
		}

		patched = true
	}

	machinePoolReplicas := int64(ptr.Deref[int32](s.scope.MachinePool.Spec.Replicas, 0))
	// Decreases in replica count is handled by deleting GCPMachinePoolMachine instances in the MachinePoolScope
	if !s.scope.HasReplicasExternallyManaged(ctx) && instanceGroup.TargetSize < machinePoolReplicas {
		log.Info("Instance Group Target Size does not match the desired replicas in MachinePool, setting replicas", "instance group", instanceGroup.TargetSize, "desired replicas", machinePoolReplicas)
		// Set replicas.
		setReplicasOperation, err := s.Client.SetInstanceGroupSize(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name, machinePoolReplicas)
		if err != nil {
			log.Error(err, "Error setting instance group size")
			return patched, err
		}

		err = s.WaitUntilComputeOperationCompleted(s.scope.Project(), s.scope.Zone(), setReplicasOperation.Name)
		if err != nil {
			log.Error(err, "Error waiting for instance group size operation to complete")
			return patched, err
		}

		patched = true
	}

	return patched, nil
}

// removeOldInstanceTemplate removes the old instance templates.
func (s *Service) removeOldInstanceTemplate(ctx context.Context, instanceTemplateName string) error {
	log := log.FromContext(ctx)

	// List all instance templates.
	instanceTemplates, err := s.Client.ListInstanceTemplates(ctx, s.scope.Project())
	if err != nil {
		log.Error(err, "Error listing instance templates")
		return err
	}

	// Prepare to identify instance templates to remove.
	lastIndex := strings.LastIndex(instanceTemplateName, "-")
	if lastIndex == -1 {
		log.Error(fmt.Errorf("invalid instance template name format"), "Invalid template name", "templateName", instanceTemplateName)
		return fmt.Errorf("invalid instance template name format: %s", instanceTemplateName)
	}

	trimmedInstanceTemplateName := instanceTemplateName[:lastIndex]
	var errors []error

	for _, instanceTemplate := range instanceTemplates.Items {
		if strings.HasPrefix(instanceTemplate.Name, trimmedInstanceTemplateName) && instanceTemplate.Name != instanceTemplateName {
			log.Info("Deleting old instance template", "templateName", instanceTemplate.Name)
			_, err := s.Client.DeleteInstanceTemplate(ctx, s.scope.Project(), instanceTemplate.Name)
			if err != nil {
				log.Error(err, "Error deleting instance template", "templateName", instanceTemplate.Name)
				errors = append(errors, err)
				continue // Proceed to next template instead of returning immediately.
			}
		}
	}

	// Aggregate errors (if any).
	if len(errors) > 0 {
		return fmt.Errorf("encountered errors during deletion: %v", errors)
	}

	return nil
}

// Delete deletes the instance group.
func (s *Service) Delete(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	igDeletionOperation, err := s.DeleteInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error deleting instance group")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Wait for the instance group to be deleted
	err = s.WaitUntilOperationCompleted(s.scope.Project(), igDeletionOperation.Name)
	if err != nil {
		log.Error(err, "Error waiting for instance group deletion operation to complete")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
