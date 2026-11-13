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

package loadbalancers

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestGetExternalLoadBalancerName(t *testing.T) {
	tests := []struct {
		name   string
		lbSpec infrav1.LoadBalancerSpec
		want   string
	}{
		{
			name: "returns custom name when set",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					LoadBalancerCommon: infrav1.LoadBalancerCommon{Name: ptr.To[string]("custom-external-lb")},
				},
			},
			want: "custom-external-lb",
		},
		{
			name: "returns default name when ExternalLoadBalancerConfig is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancerConfig: nil,
			},
			want: infrav1.APIServerRoleTagValue,
		},
		{
			name: "returns default name when Name is nil",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{},
			},
			want: infrav1.APIServerRoleTagValue,
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
					LoadBalancerCommon: infrav1.LoadBalancerCommon{Name: ptr.To[string]("custom-internal-lb")},
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
				InternalLoadBalancer: &infrav1.LoadBalancer{},
			},
			want: infrav1.InternalRoleTagValue,
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
			name:           "handles empty instance groups",
			instancegroups: []*compute.InstanceGroup{},
			mode:           loadBalancingModeUtilization,
			maxConnections: 0,
			want:           []*compute.Backend{},
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

func TestService_ensureRegionalProxyOnlySubnet(t *testing.T) {
	tests := []struct {
		name       string
		subnets    infrav1.Subnets
		region     string
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "proxy-only subnet in same region is valid",
			subnets: infrav1.Subnets{
				infrav1.SubnetSpec{Name: "proxy", Region: "us-central1", Purpose: ptr.To("REGIONAL_MANAGED_PROXY")},
				infrav1.SubnetSpec{Name: "workload", Region: "us-central1", Purpose: ptr.To("PRIVATE_RFC_1918")},
			},
			region: "us-central1",
		},
		{
			name: "proxy-only subnet with empty region defers to cluster region and is valid",
			subnets: infrav1.Subnets{
				infrav1.SubnetSpec{Name: "proxy", Purpose: ptr.To("REGIONAL_MANAGED_PROXY")},
			},
			region: "us-central1",
		},
		{
			name: "no proxy-only subnet returns actionable error",
			subnets: infrav1.Subnets{
				infrav1.SubnetSpec{Name: "control-plane", Region: "us-central1", Purpose: ptr.To("PRIVATE_RFC_1918")},
			},
			region:     "us-central1",
			wantErr:    true,
			wantErrSub: "REGIONAL_MANAGED_PROXY",
		},
		{
			name: "proxy-only subnet in a different region is not accepted",
			subnets: infrav1.Subnets{
				infrav1.SubnetSpec{Name: "proxy", Region: "us-east1", Purpose: ptr.To("REGIONAL_MANAGED_PROXY")},
			},
			region:     "us-central1",
			wantErr:    true,
			wantErrSub: "us-central1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			cs.GCPCluster.Spec.Region = tt.region
			cs.GCPCluster.Spec.Network.Subnets = tt.subnets
			s := New(cs)
			err = s.ensureRegionalProxyOnlySubnet()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ensureRegionalProxyOnlySubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error %q missing substring %q", err, tt.wantErrSub)
			}
		})
	}
}
