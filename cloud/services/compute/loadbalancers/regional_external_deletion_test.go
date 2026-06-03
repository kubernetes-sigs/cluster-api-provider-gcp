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
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
)

func TestService_deleteRegionalTargetTCPProxy(t *testing.T) {
	tests := []struct {
		name                       string
		scope                      func(s *scope.ClusterScope) Scope
		mockRegionalTargetTCPProxy *cloud.MockRegionTargetTcpProxies
		wantErr                    bool
		wantDeleted                bool
	}{
		{
			name:  "regional target tcp proxy exists (should delete)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.TargetTcpProxy{
							Name:     "my-cluster-apiserver",
							SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/targetTcpProxies/my-cluster-apiserver",
							Region:   "us-central1",
						},
					},
				},
			},
			wantErr:     false,
			wantDeleted: true,
		},
		{
			name:  "regional target tcp proxy does not exist (should succeed - no-op)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			},
			wantErr:     false,
			wantDeleted: false,
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

			err = s.deleteRegionalTargetTCPProxy(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.deleteRegionalTargetTCPProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify deletion
			if tt.wantDeleted {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := tt.mockRegionalTargetTCPProxy.Objects[*key]; exists {
					t.Errorf("Service.deleteRegionalTargetTCPProxy() did not delete the resource")
				}
			}
		})
	}
}

func TestService_deleteRegionalAddress(t *testing.T) {
	tests := []struct {
		name          string
		scope         func(s *scope.ClusterScope) Scope
		lbname        string
		mockAddresses *cloud.MockAddresses
		wantErr       bool
		wantDeleted   bool
	}{
		{
			name:   "regional address exists (should delete)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockAddressesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.Address{
							Name:     "my-cluster-apiserver",
							Region:   "us-central1",
							SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
						},
					},
				},
			},
			wantErr:     false,
			wantDeleted: true,
		},
		{
			name:   "regional address does not exist (should succeed - no-op)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			wantErr:     false,
			wantDeleted: false,
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

			err = s.deleteRegionalAddress(ctx, tt.lbname)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.deleteRegionalAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify deletion
			if tt.wantDeleted {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := tt.mockAddresses.Objects[*key]; exists {
					t.Errorf("Service.deleteRegionalAddress() did not delete the resource")
				}
			}
		})
	}
}

func TestService_deleteRegionalForwardingRule(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbname             string
		mockForwardingRule *cloud.MockForwardingRules
		wantErr            bool
		wantDeleted        bool
	}{
		{
			name:   "regional forwarding rule exists (should delete)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.ForwardingRule{
							Name:     "my-cluster-apiserver",
							Region:   "us-central1",
							SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/forwardingRules/my-cluster-apiserver",
						},
					},
				},
			},
			wantErr:     false,
			wantDeleted: true,
		},
		{
			name:   "regional forwarding rule does not exist (should succeed - no-op)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbname: "apiserver",
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			},
			wantErr:     false,
			wantDeleted: false,
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

			err = s.deleteRegionalForwardingRule(ctx, tt.lbname)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.deleteRegionalForwardingRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify deletion
			if tt.wantDeleted {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := tt.mockForwardingRule.Objects[*key]; exists {
					t.Errorf("Service.deleteRegionalForwardingRule() did not delete the resource")
				}
			}
		})
	}
}

func TestService_deleteRegionalExternalLoadBalancer(t *testing.T) {
	tests := []struct {
		name                       string
		scope                      func(s *scope.ClusterScope) Scope
		mockForwardingRule         *cloud.MockForwardingRules
		mockRegionalTargetTCPProxy *cloud.MockRegionTargetTcpProxies
		mockAddresses              *cloud.MockAddresses
		mockBackendService         *cloud.MockRegionBackendServices
		mockHealthCheck            *cloud.MockRegionHealthChecks
		wantErr                    bool
	}{
		{
			name:  "all regional resources exist (should delete all)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.ForwardingRule{
							Name:   "my-cluster-apiserver",
							Region: "us-central1",
						},
					},
				},
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.TargetTcpProxy{
							Name:   "my-cluster-apiserver",
							Region: "us-central1",
						},
					},
				},
			},
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockAddressesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.Address{
							Name:   "my-cluster-apiserver",
							Region: "us-central1",
						},
					},
				},
			},
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionBackendServicesObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.BackendService{
							Name:   "my-cluster-apiserver",
							Region: "us-central1",
						},
					},
				},
			},
			mockHealthCheck: &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockRegionHealthChecksObj{
					*meta.RegionalKey("my-cluster-apiserver", "us-central1"): {
						Obj: &compute.HealthCheck{
							Name:   "my-cluster-apiserver",
							Region: "us-central1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:  "no resources exist (should succeed - no-op)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			},
			mockAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			},
			mockHealthCheck: &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionHealthChecksObj{},
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
			s.regionaltargettcpproxies = tt.mockRegionalTargetTCPProxy
			s.addresses = tt.mockAddresses
			s.regionalbackendservices = tt.mockBackendService
			s.regionalhealthchecks = tt.mockHealthCheck

			err = s.deleteRegionalExternalLoadBalancer(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify all resources are deleted
			key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
			if _, exists := tt.mockForwardingRule.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete forwarding rule")
			}
			if _, exists := tt.mockRegionalTargetTCPProxy.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete target TCP proxy")
			}
			if _, exists := tt.mockAddresses.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete address")
			}
			if _, exists := tt.mockBackendService.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete backend service")
			}
			if _, exists := tt.mockHealthCheck.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete health check")
			}
		})
	}
}
