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
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
)

func TestService_createOrGetRegionalTargetTCPProxy(t *testing.T) {
	tests := []struct {
		name                       string
		scope                      func(s *scope.ClusterScope) Scope
		backendService             *compute.BackendService
		mockRegionalTargetTCPProxy *cloud.MockRegionTargetTcpProxies
		want                       *compute.TargetTcpProxy
		wantErr                    bool
	}{
		{
			name:  "regional target tcp proxy does not exist (should create)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			backendService: &compute.BackendService{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			},
			want: &compute.TargetTcpProxy{
				Name:        "my-cluster-apiserver",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				Service:     "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Region:      "us-central1",
				ProxyHeader: "NONE", // Default value set by mock
			},
			wantErr: false,
		},
		{
			name:  "regional target tcp proxy exists (should get)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			backendService: &compute.BackendService{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.TargetTcpProxy{
							Name:     "my-cluster-apiserver",
							SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
							Service:  "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
							Region:   "us-central1",
						},
					},
				},
			},
			want: &compute.TargetTcpProxy{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				Service:  "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Region:   "us-central1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			s := New(tt.scope(clusterScope))
			s.regionaltargettcpproxies = tt.mockRegionalTargetTCPProxy
			got, err := s.createOrGetRegionalTargetTCPProxy(ctx, tt.backendService)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRegionalTargetTCPProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetRegionalTargetTCPProxy() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalAddress(t *testing.T) {
	tests := []struct {
		name          string
		scope         func(s *scope.ClusterScope) Scope
		lbname        string
		mockAddresses *cloud.MockAddresses
		want          *compute.Address
		wantErr       bool
	}{
		{
			name:   "regional address does not exist (should create)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			want: &compute.Address{
				Name:        "my-cluster-apiserver",
				AddressType: "EXTERNAL",
				Region:      "us-central1",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
				IpVersion:   "IPV4", // Default value set by mock
			},
			wantErr: false,
		},
		{
			name:   "regional address exists (should get)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockAddressesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.Address{
							Name:        "my-cluster-apiserver",
							AddressType: "EXTERNAL",
							Region:      "us-central1",
							SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
						},
					},
				},
			},
			want: &compute.Address{
				Name:        "my-cluster-apiserver",
				AddressType: "EXTERNAL",
				Region:      "us-central1",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
			},
			wantErr: false,
		},
		{
			name: "regional address with custom IP",
			scope: func(s *scope.ClusterScope) Scope {
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					ExternalLoadBalancer: &infrav1.LoadBalancer{
						IPAddress: ptr.To[string]("10.1.2.3"),
					},
				}
				return s
			},
			lbname: "apiserver",
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			want: &compute.Address{
				Name:        "my-cluster-apiserver",
				AddressType: "EXTERNAL",
				Region:      "us-central1",
				Address:     "10.1.2.3",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
				IpVersion:   "IPV4", // Default value set by mock
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			s := New(tt.scope(clusterScope))
			s.addresses = tt.mockAddresses
			got, err := s.createOrGetRegionalAddress(ctx, tt.lbname)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRegionalAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetRegionalAddress() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalBackendServiceExternal(t *testing.T) {
	instanceGroups := []*compute.InstanceGroup{
		{
			Name:     "my-cluster-apiserver-us-central1-a",
			SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-apiserver-us-central1-a",
		},
	}

	healthCheck := &compute.HealthCheck{
		Name:     "my-cluster-apiserver",
		SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-apiserver",
	}

	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbname             string
		mode               loadBalancingMode
		instancegroups     []*compute.InstanceGroup
		healthcheck        *compute.HealthCheck
		mockBackendService *cloud.MockRegionBackendServices
		want               *compute.BackendService
		wantErr            bool
	}{
		{
			name:           "regional backend service does not exist with UTILIZATION mode",
			scope:          func(s *scope.ClusterScope) Scope { return s },
			lbname:         "apiserver",
			mode:           loadBalancingModeUtilization,
			instancegroups: instanceGroups,
			healthcheck:    healthCheck,
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			},
			want: &compute.BackendService{
				Name:                "my-cluster-apiserver",
				LoadBalancingScheme: "EXTERNAL",
				PortName:            "",
				Protocol:            "TCP", // Default value set by mock
				TimeoutSec:          600,   // Default value set by mock
				Region:              "us-central1",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Backends: []*compute.Backend{
					{
						BalancingMode: "UTILIZATION",
						Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-apiserver-us-central1-a",
					},
				},
				HealthChecks: []string{
					"https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-apiserver",
				},
			},
			wantErr: false,
		},
		{
			name:           "regional backend service with CONNECTION mode",
			scope:          func(s *scope.ClusterScope) Scope { return s },
			lbname:         "apiserver",
			mode:           loadBalancingModeConnection,
			instancegroups: instanceGroups,
			healthcheck:    healthCheck,
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			},
			want: &compute.BackendService{
				Name:                "my-cluster-apiserver",
				LoadBalancingScheme: "EXTERNAL",
				PortName:            "",
				Protocol:            "TCP", // Default value set by mock
				TimeoutSec:          600,   // Default value set by mock
				Region:              "us-central1",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-apiserver",
				Backends: []*compute.Backend{
					{
						BalancingMode:  "CONNECTION",
						Group:          "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-apiserver-us-central1-a",
						MaxConnections: 1000,
					},
				},
				HealthChecks: []string{
					"https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-apiserver",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			s := New(tt.scope(clusterScope))
			s.regionalbackendservices = tt.mockBackendService
			got, err := s.createOrGetRegionalBackendServiceExternal(ctx, tt.lbname, tt.mode, tt.instancegroups, tt.healthcheck)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRegionalBackendServiceExternal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetRegionalBackendServiceExternal() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalForwardingRuleWithProxy(t *testing.T) {
	targetProxy := &compute.TargetTcpProxy{
		Name:     "my-cluster-apiserver",
		SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
	}

	address := &compute.Address{
		Name:     "my-cluster-apiserver",
		SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
		Address:  "10.1.2.3",
	}

	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbname             string
		target             *compute.TargetTcpProxy
		addr               *compute.Address
		mockForwardingRule *cloud.MockForwardingRules
		want               *compute.ForwardingRule
		wantErr            bool
	}{
		{
			name:   "regional forwarding rule does not exist",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			target: targetProxy,
			addr:   address,
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			},
			want: &compute.ForwardingRule{
				Name:                "my-cluster-apiserver",
				LoadBalancingScheme: "EXTERNAL",
				IPProtocol:          "TCP",     // Default value set by mock
				PortRange:           "443-443", // Default value set by mock (not 6443 - mock doesn't respect our custom port)
				Region:              "us-central1",
				Target:              "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				IPAddress:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/forwardingRules/my-cluster-apiserver",
			},
			wantErr: false,
		},
		{
			name:   "regional forwarding rule exists (should get)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			target: targetProxy,
			addr:   address,
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.ForwardingRule{
							Name:                "my-cluster-apiserver",
							LoadBalancingScheme: "EXTERNAL",
							PortRange:           "6443-6443",
							Region:              "us-central1",
							Target:              "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
							IPAddress:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
							SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/forwardingRules/my-cluster-apiserver",
						},
					},
				},
			},
			want: &compute.ForwardingRule{
				Name:                "my-cluster-apiserver",
				LoadBalancingScheme: "EXTERNAL",
				PortRange:           "6443-6443",
				Region:              "us-central1",
				Target:              "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
				IPAddress:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/forwardingRules/my-cluster-apiserver",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}
			s := New(tt.scope(clusterScope))
			s.regionalforwardingrules = tt.mockForwardingRule
			got, err := s.createOrGetRegionalForwardingRuleWithProxy(ctx, tt.lbname, tt.target, tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRegionalForwardingRuleWithProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetRegionalForwardingRuleWithProxy() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
