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

package scope

import (
	"testing"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	firewallutil "sigs.k8s.io/cluster-api-provider-gcp/util/firewall"
)

func unnamedRule(port string) infrav1.FirewallRule {
	return infrav1.FirewallRule{
		Description: "test rule",
		Allowed: []infrav1.FirewallDescriptor{
			{
				IPProtocol: infrav1.FirewallProtocolTCP,
				Ports:      []string{port},
			},
		},
		Direction: infrav1.FirewallRuleDirectionIngress,
		Priority:  1000,
	}
}

// TestCreateFirewallRulesNamesUnnamedRulesLikeTheWebhook pins the two places a name can
// be generated to the same result. Clusters owned by a ClusterClass topology are not
// defaulted by the webhook, so they are named here instead, and a rule that is named
// differently by the two paths would be deleted and recreated whenever the object
// moves between them.
func TestCreateFirewallRulesNamesUnnamedRulesLikeTheWebhook(t *testing.T) {
	rules := []infrav1.FirewallRule{unnamedRule("443"), unnamedRule("8443")}

	specs, err := createFirewallRules("my-cluster", "my-network", infrav1.RulesManagementUnmanaged, rules)
	if err != nil {
		t.Fatalf("createFirewallRules() error = %v", err)
	}
	if len(specs) != len(rules) {
		t.Fatalf("createFirewallRules() returned %d rules, want %d", len(specs), len(rules))
	}

	defaulted := []infrav1.FirewallRule{unnamedRule("443"), unnamedRule("8443")}
	if err := firewallutil.DefaultRuleNames(defaulted, "my-cluster"); err != nil {
		t.Fatalf("DefaultRuleNames() error = %v", err)
	}

	for i := range defaulted {
		if specs[i].Name != defaulted[i].Name {
			t.Errorf("createFirewallRules() named rule %d %q, want %q", i, specs[i].Name, defaulted[i].Name)
		}
	}
}

// TestCreateFirewallRulesIsStableAcrossCalls guards the reconciler, which looks rules up
// by name: a name that changed between reconciles would orphan the rule created by the
// previous one.
func TestCreateFirewallRulesIsStableAcrossCalls(t *testing.T) {
	rules := []infrav1.FirewallRule{unnamedRule("443")}

	first, err := createFirewallRules("my-cluster", "my-network", infrav1.RulesManagementUnmanaged, rules)
	if err != nil {
		t.Fatalf("createFirewallRules() error = %v", err)
	}

	second, err := createFirewallRules("my-cluster", "my-network", infrav1.RulesManagementUnmanaged, rules)
	if err != nil {
		t.Fatalf("createFirewallRules() error = %v", err)
	}

	if first[0].Name != second[0].Name {
		t.Errorf("createFirewallRules() returned %q and then %q for the same rule", first[0].Name, second[0].Name)
	}
}

// TestCreateFirewallRulesDoesNotMutateTheSpec makes sure generating a name leaves the
// rules owned by the GCPCluster object untouched.
func TestCreateFirewallRulesDoesNotMutateTheSpec(t *testing.T) {
	rules := []infrav1.FirewallRule{unnamedRule("443")}

	if _, err := createFirewallRules("my-cluster", "my-network", infrav1.RulesManagementUnmanaged, rules); err != nil {
		t.Fatalf("createFirewallRules() error = %v", err)
	}

	if rules[0].Name != "" {
		t.Errorf("createFirewallRules() wrote %q back into the spec", rules[0].Name)
	}
}
