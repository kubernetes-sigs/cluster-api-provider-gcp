/*
Copyright 2021 The Kubernetes Authors.

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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"google.golang.org/api/compute/v1"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/gcp"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconcile machine instance.
func (s *Service) Reconcile(ctx context.Context) (*meta.Key, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling instanceTemplate resources")
	instanceTemplate, instanceTemplateKey, err := s.createOrGetInstanceTemplate(ctx)
	if err != nil {
		return nil, err
	}
	log.V(2).Info("binding to instanceTemplate", "selfLink", instanceTemplate.SelfLink)

	// s.scope.SetProviderID()
	// s.scope.SetAddresses(addresses)
	// s.scope.SetInstanceStatus(infrav1.InstanceStatus(instance.Status))

	// if s.scope.IsControlPlane() {
	// 	if err := s.registerControlPlaneInstance(ctx, instance); err != nil {
	// 		return err
	// 	}
	// }

	return instanceTemplateKey, nil
}

// Delete delete machine instance.
func (s *Service) Delete(ctx context.Context) error {
	log := log.FromContext(ctx)

	baseKey, err := s.scope.BaseInstanceTemplateResourceName()
	if err != nil {
		return err
	}

	selfLink := gcp.FormatKey("instanceTemplates", baseKey)
	log = log.WithValues("instanceTemplatesPrefix", selfLink)

	log.Info("Deleting instanceTemplate resources")

	log.V(2).Info("Looking for instanceTemplates for deletion")
	// TODO: Create filter
	var predicate *filter.F
	instanceTemplates, err := s.instanceTemplates.List(ctx, predicate)
	if err != nil {
		log.Error(err, "looking for instanceTemplates for deletion")
		return err
	}

	var errs []error
	for _, instanceTemplate := range instanceTemplates {
		log.V(2).Info("found instanceTemplate; will delete", "selfLink", instanceTemplate.SelfLink)

		// TODO: Verify cluster name through metadata

		// if s.scope.IsControlPlane() {
		// 	if err := s.deregisterControlPlaneInstance(ctx, instance); err != nil {
		// 		return err
		// 	}
		// }

		instanceName := instanceTemplate.Name
		log.V(2).Info("Deleting instanceTemplate", "instanceType", instanceTemplate.SelfLink)
		key := meta.GlobalKey(instanceName)
		if err := s.instanceTemplates.Delete(ctx, key); err != nil {
			if gcperrors.IsNotFound(err) {
				log.V(2).Info("instanceTemplate not found for deletion", "instanceTemplate", instanceTemplate.SelfLink)
			} else {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	joined := errors.Join(errs...)
	log.Error(joined, "failed to delete instanceTemplates")
	return joined
}

func (s *Service) createOrGetInstanceTemplate(ctx context.Context) (*compute.InstanceTemplate, *meta.Key, error) {
	log := log.FromContext(ctx)

	baseKey, err := s.scope.BaseInstanceTemplateResourceName()
	if err != nil {
		return nil, nil, err
	}

	desired, err := s.scope.InstanceTemplateResource(ctx)
	if err != nil {
		return nil, nil, err
	}

	desiredJSON, err := json.Marshal(desired)
	if err != nil {
		return nil, nil, fmt.Errorf("marshalling instance template to json: %w", err)
	}
	encoded := append([]byte(baseKey.Name), desiredJSON...)
	hash := sha256.Sum256(encoded)
	hashHex := hex.EncodeToString(hash[:])

	namePrefix := baseKey.Name
	suffix := hashHex[:16]
	name := namePrefix + suffix

	// TODO: Support regional templates?
	instanceTemplateKey := meta.GlobalKey(name)

	selfLink := gcp.FormatKey("instanceTemplates", baseKey)
	log = log.WithValues("instanceTemplate", selfLink)

	log.V(2).Info("Looking for instanceTemplate")
	instanceTemplate, err := s.instanceTemplates.Get(ctx, instanceTemplateKey)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for instanceTemplate")
			return nil, nil, err
		}

		log.V(2).Info("Creating instanceTemplate")
		if err := s.instanceTemplates.Insert(ctx, instanceTemplateKey, desired); err != nil {
			log.Error(err, "creating instanceTemplate")
			return nil, nil, err
		}

		instanceTemplate, err = s.instanceTemplates.Get(ctx, instanceTemplateKey)
		if err != nil {
			return nil, nil, err
		}
	}

	return instanceTemplate, instanceTemplateKey, nil
}

// func (s *Service) registerControlPlaneInstance(ctx context.Context, instance *compute.Instance) error {
// 	log := log.FromContext(ctx)
// 	instancegroupName := s.scope.ControlPlaneGroupName()
// 	log.V(2).Info("Ensuring instance already registered in the instancegroup", "name", instance.Name, "instancegroup", instancegroupName)
// 	instancegroupKey := meta.ZonalKey(instancegroupName, s.scope.Zone())
// 	instanceList, err := s.instancegroups.ListInstances(ctx, instancegroupKey, &compute.InstanceGroupsListInstancesRequest{
// 		InstanceState: "RUNNING",
// 	}, filter.None)
// 	if err != nil {
// 		log.Error(err, "Error retrieving list of instances in the instancegroup", "instancegroup", instancegroupName)
// 		return err
// 	}

// 	instanceSets := sets.NewString()
// 	defer instanceSets.Delete()
// 	for _, i := range instanceList {
// 		instanceSets.Insert(i.Instance)
// 	}

// 	if !instanceSets.Has(instance.SelfLink) && instance.Status == string(infrav1.InstanceStatusRunning) {
// 		log.V(2).Info("Registering instance in the instancegroup", "name", instance.Name, "instancegroup", instancegroupName)
// 		if err := s.instancegroups.AddInstances(ctx, instancegroupKey, &compute.InstanceGroupsAddInstancesRequest{
// 			Instances: []*compute.InstanceReference{
// 				{
// 					Instance: instance.SelfLink,
// 				},
// 			},
// 		}); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func (s *Service) deregisterControlPlaneInstance(ctx context.Context, instance *compute.Instance) error {
// 	log := log.FromContext(ctx)
// 	instancegroupName := s.scope.ControlPlaneGroupName()
// 	log.V(2).Info("Ensuring instance already registered in the instancegroup", "name", instance.Name, "instancegroup", instancegroupName)
// 	instancegroupKey := meta.ZonalKey(instancegroupName, s.scope.Zone())
// 	instanceList, err := s.instancegroups.ListInstances(ctx, instancegroupKey, &compute.InstanceGroupsListInstancesRequest{
// 		InstanceState: "RUNNING",
// 	}, filter.None)
// 	if err != nil {
// 		return gcperrors.IgnoreNotFound(err)
// 	}

// 	instanceSets := sets.NewString()
// 	defer instanceSets.Delete()
// 	for _, i := range instanceList {
// 		instanceSets.Insert(i.Instance)
// 	}

// 	if len(instanceSets.List()) > 0 && instanceSets.Has(instance.SelfLink) {
// 		log.V(2).Info("Deregistering instance in the instancegroup", "name", instance.Name, "instancegroup", instancegroupName)
// 		if err := s.instancegroups.RemoveInstances(ctx, instancegroupKey, &compute.InstanceGroupsRemoveInstancesRequest{
// 			Instances: []*compute.InstanceReference{
// 				{
// 					Instance: instance.SelfLink,
// 				},
// 			},
// 		}); err != nil {
// 			return gcperrors.IgnoreNotFound(err)
// 		}
// 	}

// 	return nil
// }
