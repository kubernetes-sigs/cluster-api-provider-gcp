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

package firewall

import (
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// ruleNamePattern is the RFC1035 name pattern GCP enforces on firewall rules.
var ruleNamePattern = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

func ingressRule(port string) infrav1.FirewallRule {
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

func TestGenerateRuleNameIsDeterministic(t *testing.T) {
	rule := ingressRule("443")

	first, err := GenerateRuleName("my-cluster", rule, sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	second, err := GenerateRuleName("my-cluster", rule, sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	if first != second {
		t.Errorf("GenerateRuleName() is not deterministic: got %q and %q", first, second)
	}

	if !strings.HasPrefix(first, "my-cluster-") {
		t.Errorf("GenerateRuleName() = %q, want it prefixed with the cluster name", first)
	}

	if !ruleNamePattern.MatchString(first) {
		t.Errorf("GenerateRuleName() = %q, want a name matching %s", first, ruleNamePattern)
	}
}

func TestGenerateRuleNameDiffersPerRule(t *testing.T) {
	first, err := GenerateRuleName("my-cluster", ingressRule("443"), sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	second, err := GenerateRuleName("my-cluster", ingressRule("8443"), sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	if first == second {
		t.Errorf("GenerateRuleName() returned %q for two different rules", first)
	}
}

func TestGenerateRuleNameIgnoresTheExistingName(t *testing.T) {
	rule := ingressRule("443")

	withoutName, err := GenerateRuleName("my-cluster", rule, sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	rule.Name = "some-name"
	withName, err := GenerateRuleName("my-cluster", rule, sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	if withoutName != withName {
		t.Errorf("GenerateRuleName() = %q with a name set and %q without, want the name to be ignored", withName, withoutName)
	}
}

func TestGenerateRuleNameAvoidsTakenNames(t *testing.T) {
	rule := ingressRule("443")

	taken, err := GenerateRuleName("my-cluster", rule, sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	next, err := GenerateRuleName("my-cluster", rule, sets.New(taken))
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	if next == taken {
		t.Errorf("GenerateRuleName() = %q, want a name other than the taken one", next)
	}
}

func TestGenerateRuleNameStaysWithinTheLengthLimit(t *testing.T) {
	name, err := GenerateRuleName(strings.Repeat("a", 200), ingressRule("443"), sets.New[string]())
	if err != nil {
		t.Fatalf("GenerateRuleName() error = %v", err)
	}

	if len(name) > MaxRuleNameLength {
		t.Errorf("GenerateRuleName() = %q (%d characters), want at most %d", name, len(name), MaxRuleNameLength)
	}

	if !ruleNamePattern.MatchString(name) {
		t.Errorf("GenerateRuleName() = %q, want a name matching %s", name, ruleNamePattern)
	}
}

func TestDefaultRuleNames(t *testing.T) {
	named := ingressRule("443")
	named.Name = "explicit-name"

	rules := []infrav1.FirewallRule{named, ingressRule("8443"), ingressRule("9443")}

	if err := DefaultRuleNames(rules, "my-cluster"); err != nil {
		t.Fatalf("DefaultRuleNames() error = %v", err)
	}

	if rules[0].Name != "explicit-name" {
		t.Errorf("DefaultRuleNames() overwrote a user provided name with %q", rules[0].Name)
	}

	seen := sets.New[string]()
	for _, rule := range rules {
		if rule.Name == "" {
			t.Fatal("DefaultRuleNames() left a rule without a name")
		}
		if seen.Has(rule.Name) {
			t.Errorf("DefaultRuleNames() assigned %q twice", rule.Name)
		}
		seen.Insert(rule.Name)
	}
}

func TestRuleNamePrefix(t *testing.T) {
	tests := []struct {
		name string
		obj  metav1.Object
		want string
	}{
		{
			name: "the cluster name label is preferred",
			obj: &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-gcp-cluster",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "my-cluster"},
				},
			},
			want: "my-cluster",
		},
		{
			name: "the object name is used when the label is missing",
			obj: &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-gcp-cluster"},
			},
			want: "my-gcp-cluster",
		},
		{
			name: "the object name is used when the label is empty",
			obj: &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-gcp-cluster",
					Labels: map[string]string{clusterv1.ClusterNameLabel: ""},
				},
			},
			want: "my-gcp-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuleNamePrefix(tt.obj); got != tt.want {
				t.Errorf("RuleNamePrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSkipRuleNameDefaulting(t *testing.T) {
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "objects owned by a ClusterClass topology are skipped",
			obj: &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-cluster",
					Labels: map[string]string{clusterv1.ClusterTopologyOwnedLabel: ""},
				},
			},
			want: true,
		},
		{
			name: "standalone objects are defaulted",
			obj: &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-cluster",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "my-cluster"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SkipRuleNameDefaulting(tt.obj); got != tt.want {
				t.Errorf("SkipRuleNameDefaulting() = %v, want %v", got, tt.want)
			}
		})
	}
}
