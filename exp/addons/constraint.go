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

// Package addons is CAPG's model of GKE's add-ons: which ones GCPManagedControlPlane's addonsConfig
// understands, what GKE requires before each can be enabled, and how what a cluster asks for becomes
// something GKE will accept.
//
// What a cluster asks for is resolved by Parse into a Config, which is CAPG's own representation rather
// than GKE's. A Config is what gets checked, by Validate at admission and ValidateCluster during
// reconciliation, and only then rendered into GKE's own AddonsConfig by ToGKE. Nothing outside gke.go
// deals in GKE's representation.
//
// The package is a subpackage rather than part of exp/api/v1beta1 itself because Constraint is an
// interface, which controller-gen cannot generate deepcopy functions for, and because it depends on the
// GCP SDK, which the API package deliberately does not.
package addons

import (
	"fmt"

	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

// Constraint is a documented prerequisite of an add-on. Describe is the single source of truth for how
// the requirement is worded, shared by admission errors, reconcile conditions and the generated
// documentation.
type Constraint interface {
	Describe() string
}

// SpecConstraint is decidable from what the cluster asks for, so the validating webhook enforces it at
// admission, before GKE is involved at all.
type SpecConstraint interface {
	Constraint
	CheckSpec(intent Intent) error
}

// ClusterConstraint needs the resolved picture of the cluster as well, so only the reconciler can enforce
// it.
type ClusterConstraint interface {
	Constraint
	CheckCluster(snapshot Snapshot) error
}

// Intent is what a cluster has asked for: its control plane, and the add-on configuration resolved out of
// it. It is everything available before anything is known about the cluster as it actually exists.
type Intent struct {
	ControlPlane *infrav1exp.GCPManagedControlPlane
	Config       Config
}

// Snapshot is an Intent plus the live state a ClusterConstraint needs, gathered once per evaluation and
// shared across every constraint so that the constraints themselves perform no I/O. A SpecConstraint
// receives only the Intent, so it cannot come to depend on state the webhook has no way to supply.
type Snapshot struct {
	Intent

	NodePools []NodePoolState
	// ProjectAPIsEnabled maps a GCP service name to whether it is enabled on the cluster's project. Only
	// the services CollectRequiredProjectAPIs reports get looked up, so this is empty for the common case
	// of a cluster whose add-ons need no particular API.
	ProjectAPIsEnabled map[string]bool
}

// NodePoolState is one node pool's effective shape. Each field holds the GCPManagedMachinePool's own
// value where it sets one, otherwise the value GKE resolved for the pool, otherwise the empty string.
// Constraints skip a pool for any field left empty rather than guessing at GKE's defaults.
type NodePoolState struct {
	Name        string
	MachineType string
	ImageType   string
	SandboxType string
}

// Violation is one unmet prerequisite of one enabled add-on.
type Violation struct {
	AddonKey string
	Message  string
}

// String renders the violation as a single user-facing sentence.
func (v Violation) String() string {
	return fmt.Sprintf("add-on %q %s", v.AddonKey, v.Message)
}

// Validate reports which prerequisites of the enabled add-ons are unmet, considering only those decidable
// from what the cluster asks for. A prerequisite ValidateCluster will check is not reported: it may well
// be met, and if it isn't, reconciliation says so within seconds and names it.
func Validate(intent Intent) []Violation {
	var violations []Violation
	for _, addon := range intent.Config.Addons() {
		if !addon.Enabled {
			continue
		}
		for _, constraint := range addon.Addon.SpecConstraints {
			if err := constraint.CheckSpec(intent); err != nil {
				violations = append(violations, Violation{AddonKey: addon.Addon.Key, Message: err.Error()})
			}
		}
	}
	return violations
}

// ValidateCluster reports which prerequisites of the enabled add-ons are unmet, considering only those
// needing the live state in snapshot. It does not repeat what Validate covers; a caller holding a
// Snapshot runs both.
func ValidateCluster(snapshot Snapshot) []Violation {
	var violations []Violation
	for _, addon := range snapshot.Config.Addons() {
		if !addon.Enabled {
			continue
		}
		for _, constraint := range addon.Addon.ClusterConstraints {
			if err := constraint.CheckCluster(snapshot); err != nil {
				violations = append(violations, Violation{AddonKey: addon.Addon.Key, Message: err.Error()})
			}
		}
	}
	return violations
}

// CollectRequiredProjectAPIs returns the distinct GCP service names the enabled add-ons require, so that
// a caller populating Snapshot.ProjectAPIsEnabled looks up only what this cluster actually needs rather
// than every service any add-on might ask for.
func CollectRequiredProjectAPIs(config Config) []string {
	type apiRequirer interface {
		RequiredAPI() string
	}

	seen := map[string]bool{}
	var apis []string
	for _, addon := range config.Addons() {
		if !addon.Enabled {
			continue
		}
		for _, constraint := range addon.Addon.ClusterConstraints {
			requirer, ok := constraint.(apiRequirer)
			if !ok || seen[requirer.RequiredAPI()] {
				continue
			}
			seen[requirer.RequiredAPI()] = true
			apis = append(apis, requirer.RequiredAPI())
		}
	}
	return apis
}
