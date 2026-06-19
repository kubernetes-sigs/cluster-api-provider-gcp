/*
Copyright 2024 The Kubernetes Authors.

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

package loadbalancers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestIsRegionalExternalLoadBalancer(t *testing.T) {
	tests := []struct {
		name   string
		lbType infrav1.LoadBalancerType
		want   bool
	}{
		{
			name:   "RegionalExternal returns true",
			lbType: infrav1.RegionalExternal,
			want:   true,
		},
		{
			name:   "RegionalInternalExternal returns true",
			lbType: infrav1.RegionalInternalExternal,
			want:   true,
		},
		{
			name:   "External returns false",
			lbType: infrav1.External,
			want:   false,
		},
		{
			name:   "Internal returns false",
			lbType: infrav1.Internal,
			want:   false,
		},
		{
			name:   "InternalExternal returns false",
			lbType: infrav1.InternalExternal,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRegionalExternalLoadBalancer(tt.lbType)
			if got != tt.want {
				t.Errorf("isRegionalExternalLoadBalancer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldCreateExternalLoadBalancer(t *testing.T) {
	// In this branch, shouldCreateExternalLoadBalancer targets only the
	// global external proxy LB. Regional external cases are dispatched
	// through isRegionalExternalLoadBalancer instead.
	tests := []struct {
		name   string
		lbType infrav1.LoadBalancerType
		want   bool
	}{
		{
			name:   "External returns true",
			lbType: infrav1.External,
			want:   true,
		},
		{
			name:   "InternalExternal returns true",
			lbType: infrav1.InternalExternal,
			want:   true,
		},
		{
			name:   "RegionalExternal returns false",
			lbType: infrav1.RegionalExternal,
			want:   false,
		},
		{
			name:   "RegionalInternalExternal returns false",
			lbType: infrav1.RegionalInternalExternal,
			want:   false,
		},
		{
			name:   "Internal returns false",
			lbType: infrav1.Internal,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCreateExternalLoadBalancer(tt.lbType)
			if got != tt.want {
				t.Errorf("shouldCreateExternalLoadBalancer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldCreateInternalLoadBalancer(t *testing.T) {
	tests := []struct {
		name   string
		lbType infrav1.LoadBalancerType
		want   bool
	}{
		{
			name:   "Internal returns true",
			lbType: infrav1.Internal,
			want:   true,
		},
		{
			name:   "InternalExternal returns true",
			lbType: infrav1.InternalExternal,
			want:   true,
		},
		{
			name:   "RegionalInternalExternal returns true",
			lbType: infrav1.RegionalInternalExternal,
			want:   true,
		},
		{
			name:   "External returns false",
			lbType: infrav1.External,
			want:   false,
		},
		{
			name:   "RegionalExternal returns false",
			lbType: infrav1.RegionalExternal,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCreateInternalLoadBalancer(tt.lbType)
			if got != tt.want {
				t.Errorf("shouldCreateInternalLoadBalancer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInternalLoadBalancerName(t *testing.T) {
	tests := []struct {
		name   string
		lbSpec infrav1.LoadBalancerSpec
		want   string
	}{
		{
			name: "returns custom name when set",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: &infrav1.LoadBalancer{
					Name: ptr.To[string]("custom-internal-lb"),
				},
			},
			want: "custom-internal-lb",
		},
		{
			name: "returns default name when InternalLoadBalancer is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: nil,
			},
			want: infrav1.InternalRoleTagValue,
		},
		{
			name: "returns default name when Name is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: &infrav1.LoadBalancer{
					Name: nil,
				},
			},
			want: infrav1.InternalRoleTagValue,
		},
		{
			name:   "returns default name for empty spec",
			lbSpec: infrav1.LoadBalancerSpec{},
			want:   infrav1.InternalRoleTagValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getInternalLoadBalancerName(tt.lbSpec)
			if got != tt.want {
				t.Errorf("getInternalLoadBalancerName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLoadBalancingMode(t *testing.T) {
	// In this branch, getLoadBalancingMode only services the global backend
	// service path (createOrGetBackendService). InternalExternal must pair
	// CONNECTION mode with the internal proxy LB; all other inputs that
	// reach the global path use UTILIZATION. The regional backend service
	// path always uses CONNECTION directly without consulting this helper.
	tests := []struct {
		name   string
		lbType infrav1.LoadBalancerType
		want   loadBalancingMode
	}{
		{
			name:   "InternalExternal returns CONNECTION mode",
			lbType: infrav1.InternalExternal,
			want:   loadBalancingModeConnection,
		},
		{
			name:   "External returns UTILIZATION mode",
			lbType: infrav1.External,
			want:   loadBalancingModeUtilization,
		},
		{
			name:   "Internal returns UTILIZATION mode",
			lbType: infrav1.Internal,
			want:   loadBalancingModeUtilization,
		},
		{
			name:   "RegionalExternal returns UTILIZATION mode",
			lbType: infrav1.RegionalExternal,
			want:   loadBalancingModeUtilization,
		},
		{
			name:   "RegionalInternalExternal returns UTILIZATION mode",
			lbType: infrav1.RegionalInternalExternal,
			want:   loadBalancingModeUtilization,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLoadBalancingMode(tt.lbType)
			if got != tt.want {
				t.Errorf("getLoadBalancingMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateBackends(t *testing.T) {
	const (
		igZoneASelfLink = "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a"
		igZoneBSelfLink = "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-b/instanceGroups/ig-zone-b"
	)
	instanceGroups := []*compute.InstanceGroup{
		{
			Name:     "ig-zone-a",
			SelfLink: igZoneASelfLink,
		},
		{
			Name:     "ig-zone-b",
			SelfLink: igZoneBSelfLink,
		},
	}

	tests := []struct {
		name           string
		instancegroups []*compute.InstanceGroup
		mode           loadBalancingMode
		want           []*compute.Backend
	}{
		{
			name:           "creates backends with UTILIZATION mode",
			instancegroups: instanceGroups,
			mode:           loadBalancingModeUtilization,
			want: []*compute.Backend{
				{
					BalancingMode: string(loadBalancingModeUtilization),
					Group:         igZoneASelfLink,
				},
				{
					BalancingMode: string(loadBalancingModeUtilization),
					Group:         igZoneBSelfLink,
				},
			},
		},
		{
			name:           "creates backends with CONNECTION mode and MaxConnections",
			instancegroups: instanceGroups,
			mode:           loadBalancingModeConnection,
			want: []*compute.Backend{
				{
					BalancingMode:  string(loadBalancingModeConnection),
					Group:          igZoneASelfLink,
					MaxConnections: 1000,
				},
				{
					BalancingMode:  string(loadBalancingModeConnection),
					Group:          igZoneBSelfLink,
					MaxConnections: 1000,
				},
			},
		},
		{
			name:           "handles empty instance groups",
			instancegroups: []*compute.InstanceGroup{},
			mode:           loadBalancingModeUtilization,
			want:           []*compute.Backend{},
		},
		{
			name: "handles single instance group",
			instancegroups: []*compute.InstanceGroup{
				{
					Name:     "ig-zone-a",
					SelfLink: igZoneASelfLink,
				},
			},
			mode: loadBalancingModeUtilization,
			want: []*compute.Backend{
				{
					BalancingMode: string(loadBalancingModeUtilization),
					Group:         igZoneASelfLink,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createBackends(tt.instancegroups, tt.mode)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("createBackends() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
