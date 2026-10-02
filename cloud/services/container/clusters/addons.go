/*
Copyright 2026 The Kubernetes Authors.

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

package clusters

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/container/apiv1/containerpb"
	"cloud.google.com/go/serviceusage/apiv1/serviceusagepb"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1/addons"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// addonsConfig resolves the add-on configuration the control plane asks for. Anything CAPG doesn't
// recognize is left out rather than reported: the validating webhook is the enforcement point for that,
// not this conversion.
func (s *Service) addonsConfig() addons.Config {
	config, _ := addons.Parse(s.scope.GCPManagedControlPlane.Spec.AddonsConfig, field.NewPath("spec", "AddonsConfig"))
	return config
}

// checkAddonPrerequisites verifies that GKE will accept the add-ons the cluster asks for before anything
// is sent to it, so that an unmet prerequisite is reported on the GCPManagedControlPlane straight away
// rather than arriving as an opaque GKE rejection minutes later.
//
// It only does the work when there is something new to judge: a cluster being created, or add-ons that
// differ from what the cluster already has. A cluster already in the state it asked for has nothing left
// to prove, which also keeps the project API lookup off the common reconcile path.
func (s *Service) checkAddonPrerequisites(ctx context.Context, existing *containerpb.Cluster) error {
	log := log.FromContext(ctx)

	config := s.addonsConfig()
	if len(config.Addons()) == 0 {
		log.V(4).Info("No add-ons configured, skipping prerequisite checks")
		return nil
	}
	if existing != nil {
		if changed, _ := config.DiffGKE(existing.GetAddonsConfig()); !changed {
			log.V(4).Info("Add-ons match the cluster, skipping prerequisite checks")
			return nil
		}
	}

	nodePools, err := s.nodePoolStates(ctx, existing)
	if err != nil {
		return err
	}
	projectAPIs, err := s.resolveProjectAPIs(ctx, config)
	if err != nil {
		return err
	}

	snapshot := addons.Snapshot{
		Intent:             addons.Intent{ControlPlane: s.scope.GCPManagedControlPlane, Config: config},
		NodePools:          nodePools,
		ProjectAPIsEnabled: projectAPIs,
	}

	log.V(2).Info("Checking add-on prerequisites",
		"addons", config.EnabledKeys(), "nodePools", len(nodePools), "projectAPIs", projectAPIs)

	// The prerequisites the webhook can judge are checked again here rather than taken on trust: a
	// release-channel-managed cluster has no version for the webhook to check against, and an object may
	// predate this validation existing at all.
	violations := addons.Validate(snapshot.Intent)
	violations = append(violations, addons.ValidateCluster(snapshot)...)
	if len(violations) == 0 {
		return nil
	}

	messages := make([]string, len(violations))
	for i, violation := range violations {
		messages[i] = violation.String()
	}
	log.V(2).Info("Add-on prerequisites not met", "violations", messages)

	return fmt.Errorf("add-on prerequisites not met: %s", strings.Join(messages, "; "))
}

// nodePoolStates describes the cluster's node pools as the add-on constraints need to see them: what each
// GCPManagedMachinePool asks for, falling back to what GKE resolved for the pool where it asks for
// nothing. A field neither of them settles is left empty, and constraints skip the pool for it rather
// than guessing at GKE's defaults.
//
// On the create path there is no cluster to fall back to, so a pool that leaves a field to GKE goes
// unchecked until the next reconciliation.
func (s *Service) nodePoolStates(ctx context.Context, existing *containerpb.Cluster) ([]addons.NodePoolState, error) {
	managedPools, _, err := s.scope.GetAllNodePools(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching node pools: %w", err)
	}
	return mergeNodePoolStates(managedPools, existing), nil
}

// mergeNodePoolStates combines what each GCPManagedMachinePool asks for with what GKE resolved for it.
func mergeNodePoolStates(managedPools []infrav1exp.GCPManagedMachinePool, existing *containerpb.Cluster) []addons.NodePoolState {
	resolved := make(map[string]*containerpb.NodeConfig, len(existing.GetNodePools()))
	for _, nodePool := range existing.GetNodePools() {
		resolved[nodePool.GetName()] = nodePool.GetConfig()
	}

	states := make([]addons.NodePoolState, 0, len(managedPools))
	for _, managedPool := range managedPools {
		name := managedPool.Spec.NodePoolName
		if name == "" {
			name = managedPool.Name
		}
		config := resolved[name]

		states = append(states, addons.NodePoolState{
			Name: name,
			// InstanceType wins over MachineType when both are set, matching ConvertToSdkNodePool.
			MachineType: firstSet(
				ptr.Deref(managedPool.Spec.InstanceType, ""),
				//nolint:staticcheck // deprecated, but still honoured for pools that already set it
				ptr.Deref(managedPool.Spec.MachineType, ""),
				config.GetMachineType()),
			ImageType: firstSet(ptr.Deref(managedPool.Spec.ImageType, ""), config.GetImageType()),
			SandboxType: firstSet(
				ptr.Deref(managedPool.Spec.NodeSecurity.SandboxType, ""),
				sandboxType(config)),
		})
	}
	return states
}

// resolveProjectAPIs reports which of the GCP services the enabled add-ons need are turned on for the
// cluster's project. Only the services an add-on actually names are asked about, so a cluster whose
// add-ons need none makes no call at all.
func (s *Service) resolveProjectAPIs(ctx context.Context, config addons.Config) (map[string]bool, error) {
	required := addons.CollectRequiredProjectAPIs(config)
	if len(required) == 0 {
		return nil, nil
	}

	log := log.FromContext(ctx)
	project := s.scope.GCPManagedControlPlane.Spec.Project
	parent := "projects/" + project
	names := make([]string, len(required))
	for i, api := range required {
		names[i] = fmt.Sprintf("%s/services/%s", parent, api)
	}

	log.V(2).Info("Checking whether project APIs are enabled", "project", project, "apis", required)
	services, err := s.scope.ServiceUsageClient().BatchGetServices(ctx, &serviceusagepb.BatchGetServicesRequest{
		Parent: parent,
		Names:  names,
	})
	if err != nil {
		return nil, fmt.Errorf("checking which APIs are enabled on project %s: %w", project, err)
	}

	enabled := make(map[string]bool, len(services.GetServices()))
	for _, service := range services.GetServices() {
		// Service Usage answers with the fully qualified name; add-ons name the service on its own.
		if _, api, found := strings.Cut(service.GetName(), "/services/"); found {
			enabled[api] = service.GetState() == serviceusagepb.State_ENABLED
		}
	}
	log.V(4).Info("Project API states", "states", enabled)

	return enabled, nil
}

// markAddonPrerequisiteNotMet reports an unmet prerequisite on each of the conditions a reconciliation
// that cannot proceed should fail.
func (s *Service) markAddonPrerequisiteNotMet(err error, conditions ...clusterv1beta1.ConditionType) {
	for _, condition := range conditions {
		v1beta1conditions.MarkFalse(s.scope.ConditionSetter(), condition,
			infrav1exp.GKEControlPlaneAddonPrerequisiteNotMetReason, clusterv1beta1.ConditionSeverityError, "%s", err)
	}
}

// sandboxType is the sandbox a resolved node pool runs, or the empty string where it runs none.
func sandboxType(config *containerpb.NodeConfig) string {
	if config.GetSandboxConfig() == nil {
		return ""
	}
	return config.GetSandboxConfig().GetType().String()
}

// firstSet returns the first value that is set, so that what a GCPManagedMachinePool asks for wins over
// what GKE resolved, and a field neither settles stays empty.
func firstSet(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
