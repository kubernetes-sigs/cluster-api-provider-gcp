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

func TestGetExternalLoadBalancerName(t *testing.T) {
	tests := []struct {
		name   string
		lbSpec infrav1.LoadBalancerSpec
		want   string
	}{
		{
			name: "returns custom name when set",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancer: &infrav1.LoadBalancer{
					Name: ptr.To[string]("custom-external-lb"),
				},
			},
			want: "custom-external-lb",
		},
		{
			name: "returns default name when ExternalLoadBalancer is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancer: nil,
			},
			want: infrav1.APIServerRoleTagValue,
		},
		{
			name: "returns default name when Name is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancer: &infrav1.LoadBalancer{
					Name: nil,
				},
			},
			want: infrav1.APIServerRoleTagValue,
		},
		{
			name:   "returns default name for empty spec",
			lbSpec: infrav1.LoadBalancerSpec{},
			want:   infrav1.APIServerRoleTagValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExternalLoadBalancerName(tt.lbSpec)
			if got != tt.want {
				t.Errorf("getExternalLoadBalancerName() = %v, want %v", got, tt.want)
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
			name:   "RegionalInternalExternal returns UTILIZATION mode",
			lbType: infrav1.RegionalInternalExternal,
			want:   loadBalancingModeUtilization,
		},
		{
			name:   "RegionalExternal returns UTILIZATION mode",
			lbType: infrav1.RegionalExternal,
			want:   loadBalancingModeUtilization,
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
	instanceGroups := []*compute.InstanceGroup{
		{
			Name:     "ig-zone-a",
			SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
		},
		{
			Name:     "ig-zone-b",
			SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-b/instanceGroups/ig-zone-b",
		},
	}

	tests := []struct {
		name           string
		instancegroups []*compute.InstanceGroup
		mode           loadBalancingMode
		maxConnections int64
		want           []*compute.Backend
	}{
		{
			name:           "creates backends with UTILIZATION mode",
			instancegroups: instanceGroups,
			mode:           loadBalancingModeUtilization,
			maxConnections: 0,
			want: []*compute.Backend{
				{
					BalancingMode: "UTILIZATION",
					Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
				},
				{
					BalancingMode: "UTILIZATION",
					Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-b/instanceGroups/ig-zone-b",
				},
			},
		},
		{
			name:           "creates backends with CONNECTION mode and MaxConnections",
			instancegroups: instanceGroups,
			mode:           loadBalancingModeConnection,
			maxConnections: 1000,
			want: []*compute.Backend{
				{
					BalancingMode:  "CONNECTION",
					Group:          "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
					MaxConnections: 1000,
				},
				{
					BalancingMode:  "CONNECTION",
					Group:          "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-b/instanceGroups/ig-zone-b",
					MaxConnections: 1000,
				},
			},
		},
		{
			name:           "CONNECTION mode without maxConnections (for INTERNAL backend service)",
			instancegroups: instanceGroups,
			mode:           loadBalancingModeConnection,
			maxConnections: 0,
			want: []*compute.Backend{
				{
					BalancingMode: "CONNECTION",
					Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
				},
				{
					BalancingMode: "CONNECTION",
					Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-b/instanceGroups/ig-zone-b",
				},
			},
		},
		{
			name:           "handles empty instance groups",
			instancegroups: []*compute.InstanceGroup{},
			mode:           loadBalancingModeUtilization,
			maxConnections: 0,
			want:           []*compute.Backend{},
		},
		{
			name: "handles single instance group",
			instancegroups: []*compute.InstanceGroup{
				{
					Name:     "ig-zone-a",
					SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
				},
			},
			mode:           loadBalancingModeUtilization,
			maxConnections: 0,
			want: []*compute.Backend{
				{
					BalancingMode: "UTILIZATION",
					Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/ig-zone-a",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createBackends(tt.instancegroups, tt.mode, tt.maxConnections)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("createBackends() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
