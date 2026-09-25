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

// Package firewall implements helpers shared by the firewall rule webhooks and
// the firewall rule reconciler.
package firewall

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/hash"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	// MaxRuleNameLength is the maximum length of a firewall rule name accepted by GCP.
	MaxRuleNameLength = 63

	// hashLength is the number of generated characters appended to the prefix of a
	// firewall rule name that was not provided by the user.
	hashLength = 8

	// maxNameAttempts bounds the number of names generated for a single rule while
	// looking for one that is not already taken.
	maxNameAttempts = 10
)

// SkipRuleNameDefaulting reports whether generated firewall rule names must be kept
// out of the spec of obj. The ClusterClass topology controller server-side applies the
// objects it owns from their template, so a name written by a defaulting webhook is
// attributed to that controller's field manager and removed again by its next apply,
// leaving the webhook and the topology controller overwriting each other. The firewall
// reconciler derives the same names for those clusters without touching the spec.
func SkipRuleNameDefaulting(obj metav1.Object) bool {
	_, owned := obj.GetLabels()[clusterv1.ClusterTopologyOwnedLabel]

	return owned
}

// RuleNamePrefix returns the prefix that generated firewall rule names are built
// from. The reconciler prefixes rule names with the name of the owning cluster, so
// the cluster name label is preferred and the name of the infrastructure object
// itself is only a fallback for objects that are not labelled yet.
func RuleNamePrefix(obj metav1.Object) string {
	if clusterName := obj.GetLabels()[clusterv1.ClusterNameLabel]; clusterName != "" {
		return clusterName
	}

	return obj.GetName()
}

// DefaultRuleNames assigns a generated name to every rule in rules that does not
// have one. The rules are modified in place.
func DefaultRuleNames(rules []infrav1.FirewallRule, prefix string) error {
	taken := TakenRuleNames(rules)

	for i := range rules {
		if rules[i].Name != "" {
			continue
		}

		name, err := GenerateRuleName(prefix, rules[i], taken)
		if err != nil {
			return err
		}

		rules[i].Name = name
		taken.Insert(name)
	}

	return nil
}

// TakenRuleNames returns the names already in use by rules.
func TakenRuleNames(rules []infrav1.FirewallRule) sets.Set[string] {
	taken := sets.New[string]()
	for _, rule := range rules {
		if rule.Name != "" {
			taken.Insert(rule.Name)
		}
	}

	return taken
}

// GenerateRuleName returns a name for a firewall rule that does not specify one. The
// name is the prefix followed by a hash of the rule, truncated so that it never
// exceeds MaxRuleNameLength characters. The name is derived rather than random
// because the reconciler looks rules up by name: a name that changed between
// reconciles would orphan the rule created by the previous one. Names present in
// taken are never returned.
func GenerateRuleName(prefix string, rule infrav1.FirewallRule, taken sets.Set[string]) (string, error) {
	seed, err := ruleSeed(rule)
	if err != nil {
		return "", err
	}

	prefix = truncatePrefix(prefix)
	for attempt := range maxNameAttempts {
		suffix, err := hash.Base36TruncatedHash(fmt.Sprintf("%s/%s/%d", prefix, seed, attempt), hashLength)
		if err != nil {
			return "", errors.Wrap(err, "hashing firewall rule")
		}

		name := fmt.Sprintf("%s-%s", prefix, suffix)
		if !taken.Has(name) {
			return name, nil
		}
	}

	return "", errors.Errorf("unable to generate an unused name for firewall rule with prefix %q", prefix)
}

// ruleSeed returns the bytes that identify a rule independently of its name. Two
// rules with the same seed are given the same generated name.
func ruleSeed(rule infrav1.FirewallRule) ([]byte, error) {
	// The name is what is being generated, so it cannot contribute to the hash.
	rule.Name = ""
	seed, err := json.Marshal(rule)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling firewall rule")
	}

	return seed, nil
}

// ValidateRules reports the rules that cannot be told apart from an earlier rule.
//
// Two rules sharing a name describe a single rule in GCP, so the second silently
// replaces the first. Two unnamed rules that are otherwise identical seed the name
// generator identically, and it can only work around that collision maxNameAttempts
// times before it runs out of names and fails the reconcile. Rejecting both at
// admission keeps either from reaching the reconciler.
func ValidateRules(rules []infrav1.FirewallRule, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	names := sets.New[string]()
	// Only unnamed rules are compared by content, because they are the only ones the
	// generator has to name. Identical rules that carry distinct names are redundant
	// but unambiguous, and rejecting them would break specs that work today.
	seeds := map[string]int{}

	for i, rule := range rules {
		if rule.Name != "" {
			if names.Has(rule.Name) {
				allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), rule.Name))
			}
			names.Insert(rule.Name)

			continue
		}

		seed, err := ruleSeed(rule)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Index(i), err))

			continue
		}

		if first, duplicate := seeds[string(seed)]; duplicate {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), rule,
				fmt.Sprintf("rule is identical to %s and would be given the same generated name; name one of them or remove it", fldPath.Index(first))))

			continue
		}
		seeds[string(seed)] = i
	}

	return allErrs
}

// truncatePrefix shortens prefix so that a generated suffix still fits within
// MaxRuleNameLength characters.
func truncatePrefix(prefix string) string {
	if maxPrefixLength := MaxRuleNameLength - hashLength - 1; len(prefix) > maxPrefixLength {
		prefix = prefix[:maxPrefixLength]
	}

	return strings.TrimSuffix(prefix, "-")
}
