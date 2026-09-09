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
	for _, spec := range s.scope.FirewallRulesSpec() {
		log.V(2).Info("Looking firewall", "name", spec.Name)
		firewallKey := meta.GlobalKey(spec.Name)
		actual, err := s.firewalls.Get(ctx, firewallKey)
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error looking up firewall rule", "name", spec.Name)
				return fmt.Errorf("getting firewall rule %s: %w", spec.Name, err)
			}

			log.Info("Creating firewall rule", "name", spec.Name)
			if err := s.firewalls.Insert(ctx, firewallKey, spec); err != nil {
				log.Error(err, "Error creating firewall rule", "name", spec.Name)
				return fmt.Errorf("creating firewall rule %s: %w", spec.Name, err)
			}

			continue
		}

		if drifted := driftedFields(spec, actual); len(drifted) > 0 {
			log.Info("Firewall rule exists but does not match its spec and is left unchanged: CAPG does not update firewall rules in place, apply the change directly in GCP",
				"name", spec.Name, "fields", drifted)
		}
	}

	return nil
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
	for _, spec := range s.scope.FirewallRulesSpec() {
		log.V(2).Info("Deleting firewall", "name", spec.Name)
		firewallKey := meta.GlobalKey(spec.Name)
		if err := s.firewalls.Delete(ctx, firewallKey); err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error deleting firewall rule", "name", spec.Name)
				return fmt.Errorf("deleting firewall rule %s: %w", spec.Name, err)
			}
		}
	}

	return nil
}
