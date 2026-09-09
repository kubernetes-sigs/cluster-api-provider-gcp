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
				ForceSendFields: []string{"Priority"},
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
				Direction:       "INGRESS",
				Priority:        900,
				ForceSendFields: []string{"Priority"},
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
