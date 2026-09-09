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

package firewalls

import (
	"slices"
	"strings"

	"google.golang.org/api/compute/v1"
)

// driftedFields returns the names of the user-configurable fields that differ between the
// desired spec and the firewall rule that currently exists in GCP. It is used for reporting
// only: CAPG does not update firewall rules in place, so a drifted rule is left alone and the
// difference is surfaced in the controller logs instead.
//
// Server-populated fields (SelfLink, Id, CreationTimestamp, ...) are not compared, and neither
// is the network, which cannot be changed without recreating the rule.
func driftedFields(spec, actual *compute.Firewall) []string {
	var drifted []string

	if spec.Description != actual.Description {
		drifted = append(drifted, "description")
	}
	if !strings.EqualFold(spec.Direction, actual.Direction) {
		drifted = append(drifted, "direction")
	}
	if spec.Disabled != actual.Disabled {
		drifted = append(drifted, "disabled")
	}
	// The default rules do not set a priority, leaving GCP to assign one, so only compare the
	// priority when the spec explicitly sends it. Otherwise every default rule looks drifted.
	if slices.Contains(spec.ForceSendFields, "Priority") && spec.Priority != actual.Priority {
		drifted = append(drifted, "priority")
	}
	if !equalUnordered(spec.SourceRanges, actual.SourceRanges) {
		drifted = append(drifted, "sourceRanges")
	}
	if !equalUnordered(spec.DestinationRanges, actual.DestinationRanges) {
		drifted = append(drifted, "destinationRanges")
	}
	if !equalUnordered(spec.SourceTags, actual.SourceTags) {
		drifted = append(drifted, "sourceTags")
	}
	if !equalUnordered(spec.TargetTags, actual.TargetTags) {
		drifted = append(drifted, "targetTags")
	}
	if !equalUnordered(allowedDescriptors(spec.Allowed), allowedDescriptors(actual.Allowed)) {
		drifted = append(drifted, "allowed")
	}
	if !equalUnordered(deniedDescriptors(spec.Denied), deniedDescriptors(actual.Denied)) {
		drifted = append(drifted, "denied")
	}

	return drifted
}

func allowedDescriptors(allowed []*compute.FirewallAllowed) []string {
	descriptors := make([]string, 0, len(allowed))
	for _, a := range allowed {
		descriptors = append(descriptors, describeProtocolPorts(a.IPProtocol, a.Ports))
	}

	return descriptors
}

func deniedDescriptors(denied []*compute.FirewallDenied) []string {
	descriptors := make([]string, 0, len(denied))
	for _, d := range denied {
		descriptors = append(descriptors, describeProtocolPorts(d.IPProtocol, d.Ports))
	}

	return descriptors
}

// describeProtocolPorts flattens a protocol and its ports into a single comparable string so
// that allow/deny lists can be compared without caring about ordering.
func describeProtocolPorts(protocol string, ports []string) string {
	sorted := slices.Clone(ports)
	slices.Sort(sorted)

	return strings.ToLower(protocol) + ":" + strings.Join(sorted, ",")
}

// equalUnordered reports whether two string slices hold the same elements, ignoring order. GCP
// does not guarantee that it returns list fields in the order they were submitted.
func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sortedA, sortedB := slices.Clone(a), slices.Clone(b)
	slices.Sort(sortedA)
	slices.Sort(sortedB)

	return slices.Equal(sortedA, sortedB)
}
