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
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
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
		log.Info("Instance group found, updating")
		err = s.patchInstanceGroup(ctx, instanceTemplateName, instanceGroup)
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

	// Re-get the instance group after updating it. This is needed to get the latest status.
	instanceGroup, err = s.Client.GetInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
	if err != nil {
		log.Error(err, "Error getting instance group")
		return ctrl.Result{}, err
	}

	instanceGroupResponse, err := s.Client.ListInstanceGroupInstances(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.GCPMachinePool.Name)
	if err != nil {
		log.Error(err, "Error listing instance group instances")
		return ctrl.Result{}, err
	}

	providerIDList := []string{}
	for _, managedInstance := range instanceGroupResponse.ManagedInstances {
		managedInstanceFmt := fmt.Sprintf("gce://%s/%s/%s", s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, managedInstance.Name)
		providerIDList = append(providerIDList, managedInstanceFmt)
	}

	// update ProviderID and ProviderId List
	s.scope.MachinePool.Spec.ProviderIDList = providerIDList
	s.scope.GCPMachinePool.Spec.ProviderID = fmt.Sprintf("gce://%s/%s/%s", s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, instanceGroup.Name)
	s.scope.GCPMachinePool.Spec.ProviderIDList = providerIDList

	log.Info("Instance group updated", "instance group", instanceGroup.Name, "instance group status", instanceGroup.Status, "instance group target size", instanceGroup.TargetSize, "instance group current size", instanceGroup.TargetSize)
	// Set the status.
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GCPMachinePoolUpdatingCondition, infrav1exp.GCPMachinePoolUpdatedReason, clusterv1.ConditionSeverityInfo, "")
	s.scope.SetReplicas(int32(instanceGroup.TargetSize))
	s.scope.MachinePool.Status.Replicas = int32(instanceGroup.TargetSize)
	s.scope.MachinePool.Status.ReadyReplicas = int32(instanceGroup.TargetSize)
	s.scope.GCPMachinePool.Status.Ready = true
	conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GCPMachinePoolReadyCondition)
	conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GCPMachinePoolCreatingCondition, infrav1exp.GCPMachinePoolUpdatedReason, clusterv1.ConditionSeverityInfo, "")

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
func (s *Service) patchInstanceGroup(ctx context.Context, instanceTemplateName string, instanceGroup *compute.InstanceGroupManager) error {
	log := log.FromContext(ctx)

	err := s.scope.ReconcileReplicas(ctx, instanceGroup)
	if err != nil {
		log.Error(err, "Error reconciling replicas")
		return err
	}

	lastSlashTemplateURI := strings.LastIndex(instanceGroup.InstanceTemplate, "/")
	fetchedInstanceTemplateName := instanceGroup.InstanceTemplate[lastSlashTemplateURI+1:]

	// Check if instance group is already using the instance template.
	if fetchedInstanceTemplateName != instanceTemplateName {
		log.Info("Instance group is not using the instance template, setting instance template", "instance group", instanceGroup.InstanceTemplate, "instance template", instanceTemplateName)
		// Set instance template.
		_, err := s.Client.SetInstanceGroupTemplate(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.InstanceGroupBuilder(instanceTemplateName))
		if err != nil {
			log.Error(err, "Error setting instance group template")
			return err
		}
	}

	// If the instance group is already using the instance template, update the instance group. Otherwise, set the instance template.
	if fetchedInstanceTemplateName == instanceTemplateName {
		log.Info("Instance group is using the instance template, updating instance group")
		instanceGroupUpdateOperation, err := s.Client.UpdateInstanceGroup(ctx, s.scope.Project(), s.scope.GCPMachinePool.Spec.Zone, s.scope.InstanceGroupBuilder(instanceTemplateName))
		if err != nil {
			log.Error(err, "Error updating instance group")
			return err
		}

		err = s.WaitUntilComputeOperationCompleted(s.scope.Project(), s.scope.Zone(), instanceGroupUpdateOperation.Name)
		if err != nil {
			log.Error(err, "Error waiting for instance group update operation to complete")
			return err
		}
	}

	return nil
}

// removeOldInstanceTemplate removes the old instance templates.
func (s *Service) removeOldInstanceTemplate(ctx context.Context, instanceTemplateName string) error {
	log := log.FromContext(ctx)
	log.Info("Starting to remove old instance templates", "templateName", instanceTemplateName)

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
			log.Info("Deleting instance template", "templateName", instanceTemplate.Name)
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
