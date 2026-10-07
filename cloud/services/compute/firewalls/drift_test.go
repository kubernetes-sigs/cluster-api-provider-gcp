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
	"reflect"
	"testing"

	"google.golang.org/api/compute/v1"
)

func TestDriftedFields(t *testing.T) {
	tests := []struct {
		name   string
		spec   *compute.Firewall
		actual *compute.Firewall
		want   []string
	}{
		{
			name: "identical rules do not drift",
			spec: &compute.Firewall{
				Description:  "allow ssh",
				Direction:    "INGRESS",
				Priority:     900,
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"ssh-enabled"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"22"}},
				},
			},
			actual: &compute.Firewall{
				Description:  "allow ssh",
				Direction:    "INGRESS",
				Priority:     900,
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"ssh-enabled"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"22"}},
				},
			},
			want: nil,
		},
		{
			// The default rules leave Priority unset, so GCP assigns 1000. Comparing it
			// unconditionally would report every default rule as drifted on every reconcile.
			name: "default rule with a server assigned priority does not drift",
			spec: &compute.Firewall{
				Direction:  "INGRESS",
				SourceTags: []string{"test-cluster-control-plane"},
				TargetTags: []string{"test-cluster-control-plane"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "all"},
				},
			},
			actual: &compute.Firewall{
				Direction:  "INGRESS",
				Priority:   1000,
				SourceTags: []string{"test-cluster-control-plane"},
				TargetTags: []string{"test-cluster-control-plane"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "all"},
				},
			},
			want: nil,
		},
		{
			// An egress rule without a destination applies to every address, and GCP stores
			// that by filling the field in. Comparing it literally would update the rule on
			// every reconcile.
			name: "egress rule with a server assigned destination does not drift",
			spec: &compute.Firewall{
				Direction: "EGRESS",
				Priority:  1000,
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			actual: &compute.Firewall{
				Direction:         "EGRESS",
				Priority:          1000,
				DestinationRanges: []string{"0.0.0.0/0"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"80"}},
				},
			},
			want: nil,
		},
		{
			name: "ingress rule with a server assigned source does not drift",
			spec: &compute.Firewall{
				Direction:  "INGRESS",
				Priority:   1000,
				TargetTags: []string{"web"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"443"}},
				},
			},
			actual: &compute.Firewall{
				Direction:    "INGRESS",
				Priority:     1000,
				SourceRanges: []string{"0.0.0.0/0"},
				TargetTags:   []string{"web"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"443"}},
				},
			},
			want: nil,
		},
		{
			// Only a range that widens the rule to everything is the one GCP fills in, so a
			// narrower range is still somebody changing the rule out of band.
			name: "narrowed range on a rule that asks for none drifts",
			spec: &compute.Firewall{
				Direction: "EGRESS",
				Priority:  1000,
			},
			actual: &compute.Firewall{
				Direction:         "EGRESS",
				Priority:          1000,
				DestinationRanges: []string{"10.0.0.0/8"},
			},
			want: []string{"destinationRanges"},
		},
		{
			name: "server populated fields are ignored",
			spec: &compute.Firewall{
				Direction: "INGRESS",
				Network:   "projects/test/global/networks/default",
			},
			actual: &compute.Firewall{
				Direction:         "INGRESS",
				Network:           "https://www.googleapis.com/compute/v1/projects/test/global/networks/default",
				Id:                12345,
				SelfLink:          "https://www.googleapis.com/compute/v1/projects/test/global/firewalls/rule",
				CreationTimestamp: "2026-01-01T00:00:00.000-08:00",
				Kind:              "compute#firewall",
			},
			want: nil,
		},
		{
			name: "reordered list fields do not drift",
			spec: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{"10.0.0.0/8", "172.16.0.0/12"},
				TargetTags:   []string{"web", "api"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"443", "80"}},
					{IPProtocol: "udp", Ports: []string{"53"}},
				},
			},
			actual: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{"172.16.0.0/12", "10.0.0.0/8"},
				TargetTags:   []string{"api", "web"},
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "udp", Ports: []string{"53"}},
					{IPProtocol: "tcp", Ports: []string{"80", "443"}},
				},
			},
			want: nil,
		},
		{
			name: "nil and empty slices are equivalent",
			spec: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: nil,
				Allowed:      nil,
				Denied:       []*compute.FirewallDenied{},
			},
			actual: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{},
				Allowed:      []*compute.FirewallAllowed{},
				Denied:       nil,
			},
			want: nil,
		},
		{
			name: "direction casing is ignored",
			spec: &compute.Firewall{
				Direction: "Ingress",
			},
			actual: &compute.Firewall{
				Direction: "INGRESS",
			},
			want: nil,
		},
		{
			name: "changed ports drift",
			spec: &compute.Firewall{
				Direction: "INGRESS",
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"22", "2222"}},
				},
			},
			actual: &compute.Firewall{
				Direction: "INGRESS",
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp", Ports: []string{"22"}},
				},
			},
			want: []string{"allowed"},
		},
		{
			name: "explicitly set priority drifts",
			spec: &compute.Firewall{
				Direction: "INGRESS",
				Priority:  900,
			},
			actual: &compute.Firewall{
				Direction: "INGRESS",
				Priority:  1000,
			},
			want: []string{"priority"},
		},
		{
			name: "multiple changed fields are all reported",
			spec: &compute.Firewall{
				Description:  "updated",
				Direction:    "EGRESS",
				Disabled:     true,
				SourceRanges: []string{"10.0.0.0/8"},
				TargetTags:   []string{"web"},
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "tcp", Ports: []string{"23"}},
				},
			},
			actual: &compute.Firewall{
				Description:       "original",
				Direction:         "INGRESS",
				Disabled:          false,
				DestinationRanges: []string{"198.51.100.0/24"},
				SourceTags:        []string{"app"},
			},
			want: []string{
				"description", "direction", "disabled", "sourceRanges",
				"destinationRanges", "sourceTags", "targetTags", "denied",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driftedFields(tt.spec, tt.actual)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("driftedFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDriftedFieldsRanges covers the source and destination of a rule on its own, because
// GCP fills an omitted range in with the range that matches everything and a comparison
// that misses that updates the rule on every reconcile for as long as it exists.
func TestDriftedFieldsRanges(t *testing.T) {
	tests := []struct {
		name   string
		spec   *compute.Firewall
		actual *compute.Firewall
		want   []string
	}{
		{
			name:   "no range on either side",
			spec:   &compute.Firewall{Direction: "EGRESS"},
			actual: &compute.Firewall{Direction: "EGRESS"},
			want:   nil,
		},
		{
			name:   "omitted destination filled in with every IPv4 address",
			spec:   &compute.Firewall{Direction: "EGRESS"},
			actual: &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"0.0.0.0/0"}},
			want:   nil,
		},
		{
			name:   "omitted source filled in with every address",
			spec:   &compute.Firewall{Direction: "INGRESS"},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"0.0.0.0/0"}},
			want:   nil,
		},
		{
			// The range fields accept IPv6, so a rule that asks for one is compared like
			// any other: the spec says what the range is and nothing is filled in.
			name:   "IPv6 range the spec asks for is unchanged",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"2001:db8::/32"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"2001:db8::/32"}},
			want:   nil,
		},
		{
			name:   "IPv6 and IPv4 ranges the spec asks for are reordered",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"10.0.0.0/8", "2001:db8::/32"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"2001:db8::/32", "10.0.0.0/8"}},
			want:   nil,
		},
		{
			name:   "IPv6 range the spec asks for changed out of band",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"2001:db8::/32"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"2001:db8:1::/48"}},
			want:   []string{"sourceRanges"},
		},
		{
			// An omitted range means every IPv4 address, which is what GCP fills it in
			// with, so a rule carrying only the IPv6 equivalent no longer covers the
			// traffic the spec asked for and is not a default left alone.
			name:   "omitted destination carrying only every IPv6 address drifts",
			spec:   &compute.Firewall{Direction: "EGRESS"},
			actual: &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"::/0"}},
			want:   []string{"destinationRanges"},
		},
		{
			name:   "range the spec asks for is unchanged",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"10.0.0.0/8"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"10.0.0.0/8"}},
			want:   nil,
		},
		{
			name:   "range the spec asks for is reordered",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"10.0.0.0/8", "172.16.0.0/12"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"172.16.0.0/12", "10.0.0.0/8"}},
			want:   nil,
		},
		{
			name:   "everything the spec asks for explicitly",
			spec:   &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"0.0.0.0/0"}},
			actual: &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"0.0.0.0/0"}},
			want:   nil,
		},
		{
			// Only a range that leaves the rule applying to everything is one GCP filled in.
			name:   "omitted destination narrowed out of band",
			spec:   &compute.Firewall{Direction: "EGRESS"},
			actual: &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"10.0.0.0/8"}},
			want:   []string{"destinationRanges"},
		},
		{
			name:   "omitted destination partly narrowed out of band",
			spec:   &compute.Firewall{Direction: "EGRESS"},
			actual: &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"0.0.0.0/0", "10.0.0.0/8"}},
			want:   []string{"destinationRanges"},
		},
		{
			name:   "range the spec asks for widened out of band",
			spec:   &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"10.0.0.0/8"}},
			actual: &compute.Firewall{Direction: "INGRESS", SourceRanges: []string{"0.0.0.0/0"}},
			want:   []string{"sourceRanges"},
		},
		{
			name:   "range the spec asks for removed out of band",
			spec:   &compute.Firewall{Direction: "EGRESS", DestinationRanges: []string{"10.0.0.0/8"}},
			actual: &compute.Firewall{Direction: "EGRESS"},
			want:   []string{"destinationRanges"},
		},
		{
			// A rule that selects its source by tag keeps an empty list, so GCP fills
			// nothing in and an empty list means the tags, not every address.
			name: "source selected by tag keeps an empty range",
			spec: &compute.Firewall{
				Direction:  "INGRESS",
				SourceTags: []string{"my-cluster-control-plane"},
			},
			actual: &compute.Firewall{
				Direction:  "INGRESS",
				SourceTags: []string{"my-cluster-control-plane"},
			},
			want: nil,
		},
		{
			// GCP takes the source of such a rule as the union of the two, so a rule may
			// ask for a tag and every address at once. Asking for it is what makes it the
			// desired state, and nothing is reverted.
			name: "source selected by tag that also asks for every address",
			spec: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{"0.0.0.0/0"},
				SourceTags:   []string{"my-cluster-control-plane"},
			},
			actual: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{"0.0.0.0/0"},
				SourceTags:   []string{"my-cluster-control-plane"},
			},
			want: nil,
		},
		{
			// Widening a rule like that to the whole internet is a change made out of band,
			// not a default GCP filled in, so it has to be reverted.
			name: "source selected by tag widened to every address out of band",
			spec: &compute.Firewall{
				Direction:  "INGRESS",
				SourceTags: []string{"my-cluster-control-plane"},
			},
			actual: &compute.Firewall{
				Direction:    "INGRESS",
				SourceRanges: []string{"0.0.0.0/0"},
				SourceTags:   []string{"my-cluster-control-plane"},
			},
			want: []string{"sourceRanges"},
		},
		{
			name: "source selected by service account widened out of band",
			spec: &compute.Firewall{
				Direction:             "INGRESS",
				SourceServiceAccounts: []string{"nodes@my-proj.iam.gserviceaccount.com"},
			},
			actual: &compute.Firewall{
				Direction:             "INGRESS",
				SourceRanges:          []string{"0.0.0.0/0"},
				SourceServiceAccounts: []string{"nodes@my-proj.iam.gserviceaccount.com"},
			},
			want: []string{"sourceRanges"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driftedFields(tt.spec, tt.actual)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("driftedFields() = %v, want %v", got, tt.want)
			}
		})
	}
}
