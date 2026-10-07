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
	"k8s.io/apimachinery/pkg/util/validation/field"
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

func TestQualifyRuleName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		rule   string
		want   string
	}{
		{
			name:   "a name without the cluster prefix is prefixed",
			prefix: "my-cluster",
			rule:   "ssh",
			want:   "my-cluster-ssh",
		},
		{
			name:   "a name that already carries the cluster prefix is left alone",
			prefix: "my-cluster",
			rule:   "my-cluster-ssh",
			want:   "my-cluster-ssh",
		},
		{
			name:   "a name longer than the limit is truncated",
			prefix: strings.Repeat("a", MaxRuleNameLength-2),
			rule:   "ssh",
			want:   strings.Repeat("a", MaxRuleNameLength-2) + "-s",
		},
		{
			name:   "a name truncated onto its separator does not end in a dash",
			prefix: strings.Repeat("a", MaxRuleNameLength-1),
			rule:   "ssh",
			want:   strings.Repeat("a", MaxRuleNameLength-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QualifyRuleName(tt.prefix, tt.rule); got != tt.want {
				t.Errorf("QualifyRuleName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestQualifyRuleNameLeavesGeneratedNamesAlone covers the cluster names that are long
// enough for GenerateRuleName to truncate them. A generated name carries the truncated
// prefix, so qualifying it against the untruncated cluster name would prefix it a second
// time, and the truncation would cut every rule of the cluster back to the same string.
func TestQualifyRuleNameLeavesGeneratedNamesAlone(t *testing.T) {
	// One character longer than the prefix a generated name can carry.
	prefix := strings.Repeat("a", MaxRuleNameLength-hashLength)

	rules := []infrav1.FirewallRule{ingressRule("443"), ingressRule("8443")}
	if err := DefaultRuleNames(rules, prefix); err != nil {
		t.Fatalf("DefaultRuleNames() error = %v", err)
	}

	for i, rule := range rules {
		if got := QualifyRuleName(prefix, rule.Name); got != rule.Name {
			t.Errorf("QualifyRuleName() renamed generated rule %d from %q to %q", i, rule.Name, got)
		}
	}
}

func TestValidateRulesAcceptsDistinctRules(t *testing.T) {
	named := ingressRule("443")
	named.Name = "https"
	renamed := ingressRule("8443")
	renamed.Name = "alt-https"

	rules := []infrav1.FirewallRule{named, renamed, ingressRule("22"), ingressRule("80")}

	if errs := ValidateRules(rules, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRules() = %v, want no errors", errs)
	}
}

func TestValidateRulesRejectsDuplicateNames(t *testing.T) {
	first := ingressRule("443")
	first.Name = "https"
	second := ingressRule("8443")
	second.Name = "https"

	errs := ValidateRules([]infrav1.FirewallRule{first, second}, field.NewPath("rules"))
	if len(errs) != 1 {
		t.Fatalf("ValidateRules() = %v, want exactly one error", errs)
	}

	if got, want := errs[0].Field, "rules[1].name"; got != want {
		t.Errorf("ValidateRules() reported field %q, want %q", got, want)
	}

	if errs[0].Type != field.ErrorTypeDuplicate {
		t.Errorf("ValidateRules() reported %v, want %v", errs[0].Type, field.ErrorTypeDuplicate)
	}
}

func TestValidateRulesRejectsIdenticalUnnamedRules(t *testing.T) {
	errs := ValidateRules([]infrav1.FirewallRule{ingressRule("443"), ingressRule("443")}, field.NewPath("rules"))
	if len(errs) != 1 {
		t.Fatalf("ValidateRules() = %v, want exactly one error", errs)
	}

	if got, want := errs[0].Field, "rules[1]"; got != want {
		t.Errorf("ValidateRules() reported field %q, want %q", got, want)
	}

	if !strings.Contains(errs[0].Detail, "rules[0]") {
		t.Errorf("ValidateRules() detail = %q, want it to name the rule that was duplicated", errs[0].Detail)
	}
}

// TestValidateRulesAllowsIdenticalRulesWithDistinctNames guards the narrowness of the
// content check: identical rules are redundant, but naming them makes them unambiguous
// and specs that do so work today.
func TestValidateRulesAllowsIdenticalRulesWithDistinctNames(t *testing.T) {
	first := ingressRule("443")
	first.Name = "https"
	second := ingressRule("443")
	second.Name = "https-too"

	if errs := ValidateRules([]infrav1.FirewallRule{first, second}, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRules() = %v, want no errors", errs)
	}
}

func TestValidateRuleUpdatesRejectsAChangedDirection(t *testing.T) {
	before := ingressRule("443")
	before.Name = "https"
	after := before
	after.Direction = infrav1.FirewallRuleDirectionEgress

	errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules"))
	if len(errs) != 1 {
		t.Fatalf("ValidateRuleUpdates() = %v, want exactly one error", errs)
	}

	if got, want := errs[0].Field, "rules[0].direction"; got != want {
		t.Errorf("ValidateRuleUpdates() reported field %q, want %q", got, want)
	}
}

func TestValidateRuleUpdatesRejectsAChangedAction(t *testing.T) {
	before := ingressRule("443")
	before.Name = "https"
	after := before
	after.Allowed, after.Denied = nil, before.Allowed

	errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules"))
	if len(errs) != 1 {
		t.Fatalf("ValidateRuleUpdates() = %v, want exactly one error", errs)
	}

	if got, want := errs[0].Field, "rules[0]"; got != want {
		t.Errorf("ValidateRuleUpdates() reported field %q, want %q", got, want)
	}
}

func TestValidateRuleUpdatesAcceptsMutableChanges(t *testing.T) {
	before := ingressRule("443")
	before.Name = "https"
	after := before
	after.Priority = 900
	after.Description = "now documented"
	after.SourceRanges = []string{"10.0.0.0/8"}

	if errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRuleUpdates() = %v, want no errors", errs)
	}
}

// TestValidateRuleUpdatesAcceptsARenamedRule covers the fix the error message asks for:
// a rule under a new name is a new rule in GCP, so it is created rather than updated and
// the immutable fields are free to change.
func TestValidateRuleUpdatesAcceptsARenamedRule(t *testing.T) {
	before := ingressRule("443")
	before.Name = "https"
	after := before
	after.Name = "https-egress"
	after.Direction = infrav1.FirewallRuleDirectionEgress

	if errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRuleUpdates() = %v, want no errors", errs)
	}
}

// TestValidateRuleUpdatesIgnoresUnnamedRules guards the exemption unnamed rules get:
// their name is derived from their contents, so changing the direction already names
// them differently and replaces the rule instead of updating it.
func TestValidateRuleUpdatesIgnoresUnnamedRules(t *testing.T) {
	before := ingressRule("443")
	after := before
	after.Direction = infrav1.FirewallRuleDirectionEgress

	if errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRuleUpdates() = %v, want no errors", errs)
	}
}

// TestValidateRuleUpdatesTreatsAnUnsetDirectionAsIngress keeps rules stored before the
// direction was defaulted from looking like they changed direction.
func TestValidateRuleUpdatesTreatsAnUnsetDirectionAsIngress(t *testing.T) {
	before := ingressRule("443")
	before.Name = "https"
	before.Direction = ""
	after := before
	after.Direction = infrav1.FirewallRuleDirectionIngress

	if errs := ValidateRuleUpdates([]infrav1.FirewallRule{before}, []infrav1.FirewallRule{after}, field.NewPath("rules")); len(errs) != 0 {
		t.Errorf("ValidateRuleUpdates() = %v, want no errors", errs)
	}
}

// TestValidateRulesRejectsWhatExhaustsTheNameGenerator ties the validation to the
// failure it exists to prevent: enough identical unnamed rules that the generator runs
// out of names to fall back on.
func TestValidateRulesRejectsWhatExhaustsTheNameGenerator(t *testing.T) {
	rules := make([]infrav1.FirewallRule, maxNameAttempts+1)
	for i := range rules {
		rules[i] = ingressRule("443")
	}

	if err := DefaultRuleNames(rules, "my-cluster"); err == nil {
		t.Fatal("DefaultRuleNames() succeeded, want it to run out of names")
	}

	// DefaultRuleNames names the rules in place, so rebuild them before validating.
	for i := range rules {
		rules[i] = ingressRule("443")
	}

	if errs := ValidateRules(rules, field.NewPath("rules")); len(errs) == 0 {
		t.Error("ValidateRules() = no errors, want the duplicates rejected")
	}
}
