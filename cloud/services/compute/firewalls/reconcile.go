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

package firewalls

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconcile cluster firewall compoenents.
func (s *Service) Reconcile(ctx context.Context) error {
	log := log.FromContext(ctx)
	if s.scope.SkipFirewallRulesManagement() {
		log.Info("Skipping firewall rule reconciliation: the cluster uses a shared VPC, so CAPG will not create, modify or delete any firewall rule",
			"project", s.scope.Project(), "hostProject", s.scope.NetworkProject())
		return nil
	}
	log.Info("Reconciling firewall resources")
	specs, err := s.scope.FirewallRulesSpec()
	if err != nil {
		log.Error(err, "Error building the firewall rule specs")
		return fmt.Errorf("building firewall rule specs: %w", err)
	}

	desired := sets.New[string]()
	for _, spec := range specs {
		desired.Insert(spec.Name)

		log.V(2).Info("Looking firewall", "name", spec.Name)
		firewallKey := meta.GlobalKey(spec.Name)
		actual, err := s.firewalls.Get(ctx, firewallKey)
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error looking up firewall rule", "name", spec.Name)
				return fmt.Errorf("getting firewall rule %s: %w", spec.Name, err)
			}

			// Record the rule before creating it. The reconciler is the only thing
			// that knows the rule is about to exist, so a restart between the insert
			// and the status patch would otherwise leave it behind with nothing
			// tracking it.
			s.recordFirewallRule(spec.Name, "")

			log.Info("Creating firewall rule", "name", spec.Name)
			if err := s.firewalls.Insert(ctx, firewallKey, spec); err != nil {
				log.Error(err, "Error creating firewall rule", "name", spec.Name)
				return fmt.Errorf("creating firewall rule %s: %w", spec.Name, err)
			}

			created, err := s.firewalls.Get(ctx, firewallKey)
			if err != nil {
				log.Error(err, "Error looking up the created firewall rule", "name", spec.Name)
				return fmt.Errorf("getting created firewall rule %s: %w", spec.Name, err)
			}
			s.recordFirewallRule(created.Name, created.SelfLink)

			continue
		}

		s.recordFirewallRule(actual.Name, actual.SelfLink)

		if drifted := driftedFields(spec, actual); len(drifted) > 0 {
			log.Info("Firewall rule drifted from spec, updating", "name", spec.Name, "fields", drifted)
			if err := s.firewalls.Update(ctx, firewallKey, spec); err != nil {
				log.Error(err, "Error updating firewall rule", "name", spec.Name)
				return fmt.Errorf("updating firewall rule %s: %w", spec.Name, err)
			}
		}
	}

	return s.prune(ctx, desired)
}

// prune deletes the firewall rules that CAPG created but that are no longer part of
// the cluster spec, for instance because the rule was renamed or removed. Rules that
// CAPG never created are not recorded in the status and are therefore never touched.
func (s *Service) prune(ctx context.Context, desired sets.Set[string]) error {
	log := log.FromContext(ctx)
	recorded := s.scope.Network().FirewallRules

	for name := range recorded {
		if desired.Has(name) {
			continue
		}

		log.Info("Deleting firewall rule that is no longer part of the cluster spec", "name", name)
		if err := s.firewalls.Delete(ctx, meta.GlobalKey(name)); err != nil && !gcperrors.IsNotFound(err) {
			// Only stop tracking a rule once it is known to be gone. Dropping it on any
			// other error would leave an orphan that nothing can find again.
			log.Error(err, "Error deleting firewall rule", "name", name)
			return fmt.Errorf("deleting firewall rule %s: %w", name, err)
		}

		delete(recorded, name)
	}

	return nil
}

// recordFirewallRule tracks a firewall rule created by CAPG in the cluster status.
func (s *Service) recordFirewallRule(name, selfLink string) {
	network := s.scope.Network()
	if network.FirewallRules == nil {
		network.FirewallRules = map[string]string{}
	}

	network.FirewallRules[name] = selfLink
}

// Delete delete cluster firewall compoenents.
func (s *Service) Delete(ctx context.Context) error {
	log := log.FromContext(ctx)
	if s.scope.SkipFirewallRulesManagement() {
		log.Info("Skipping firewall rule deletion: the cluster uses a shared VPC, so CAPG will not create, modify or delete any firewall rule",
			"project", s.scope.Project(), "hostProject", s.scope.NetworkProject())
		return nil
	}
	log.Info("Deleting firewall resources")
	recorded := s.scope.Network().FirewallRules

	// The rules recorded in the status are the ones CAPG knows it created, but
	// clusters created before the status was recorded have none, so the rules
	// derived from the spec are deleted as well.
	names := sets.KeySet(recorded)
	specs, err := s.scope.FirewallRulesSpec()
	if err != nil {
		log.Error(err, "Error building the firewall rule specs")
		return fmt.Errorf("building firewall rule specs: %w", err)
	}
	for _, spec := range specs {
		names.Insert(spec.Name)
	}

	for _, name := range sets.List(names) {
		log.V(2).Info("Deleting firewall", "name", name)
		firewallKey := meta.GlobalKey(name)
		if err := s.firewalls.Delete(ctx, firewallKey); err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error deleting firewall rule", "name", name)
				return fmt.Errorf("deleting firewall rule %s: %w", name, err)
			}
		}

		delete(recorded, name)
	}

	return nil
}
