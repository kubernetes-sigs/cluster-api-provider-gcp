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

const (
	deletionTestProjectID    = "proj-id"
	deletionTestRegion       = "us-central1"
	deletionTestLBName       = "apiserver"
	deletionTestResourceName = "my-cluster-apiserver"
)

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
			lbname: deletionTestLBName,
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.ForwardingRule{
							Name:     deletionTestResourceName,
							Region:   deletionTestRegion,
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
			lbname: deletionTestLBName,
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
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

			if tt.wantDeleted {
				key := meta.RegionalKey(deletionTestResourceName, deletionTestRegion)
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
		mockRegionalAddresses      *cloud.MockAddresses
		mockBackendService         *cloud.MockRegionBackendServices
		mockHealthCheck            *cloud.MockRegionHealthChecks
		wantErr                    bool
	}{
		{
			name:  "all regional resources exist (should delete all)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.ForwardingRule{
							Name:   deletionTestResourceName,
							Region: deletionTestRegion,
						},
					},
				},
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.TargetTcpProxy{
							Name:   deletionTestResourceName,
							Region: deletionTestRegion,
						},
					},
				},
			},
			mockRegionalAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockAddressesObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.Address{
							Name:   deletionTestResourceName,
							Region: deletionTestRegion,
						},
					},
				},
			},
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockRegionBackendServicesObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.BackendService{
							Name:   deletionTestResourceName,
							Region: deletionTestRegion,
						},
					},
				},
			},
			mockHealthCheck: &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects: map[meta.Key]*cloud.MockRegionHealthChecksObj{
					*meta.RegionalKey(deletionTestResourceName, deletionTestRegion): {
						Obj: &compute.HealthCheck{
							Name:   deletionTestResourceName,
							Region: deletionTestRegion,
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
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			},
			mockRegionalTargetTCPProxy: &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			},
			mockRegionalAddresses: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			},
			mockHealthCheck: &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: deletionTestProjectID},
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
			s.regionaladdresses = tt.mockRegionalAddresses
			s.regionalbackendservices = tt.mockBackendService
			s.regionalhealthchecks = tt.mockHealthCheck

			err = s.deleteRegionalExternalLoadBalancer(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			key := meta.RegionalKey(deletionTestResourceName, deletionTestRegion)
			if _, exists := tt.mockForwardingRule.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete forwarding rule")
			}
			if _, exists := tt.mockRegionalTargetTCPProxy.Objects[*key]; exists {
				t.Errorf("Service.deleteRegionalExternalLoadBalancer() did not delete target TCP proxy")
			}
			if _, exists := tt.mockRegionalAddresses.Objects[*key]; exists {
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
