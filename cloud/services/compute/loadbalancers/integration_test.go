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
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

// TestService_Reconcile_RegionalExternal tests the full reconciliation flow
// for RegionalExternal load balancer type.
func TestService_Reconcile_RegionalExternal(t *testing.T) {
	tests := []struct {
		name                       string
		lbType                     infrav1.LoadBalancerType
		wantRegionalTargetTCPProxy bool
		wantRegionalAddress        bool
		wantRegionalBackendService bool
		wantRegionalForwardingRule bool
		wantRegionalHealthCheck    bool
		wantErr                    bool
	}{
		{
			name:                       "RegionalExternal creates regional external LB",
			lbType:                     infrav1.RegionalExternal,
			wantRegionalTargetTCPProxy: true,
			wantRegionalAddress:        true,
			wantRegionalBackendService: true,
			wantRegionalForwardingRule: true,
			wantRegionalHealthCheck:    true,
			wantErr:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}

			// Set the load balancer type
			clusterScope.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(tt.lbType),
			}

			// Initialize mocks
			s := New(clusterScope)
			s.addresses = &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			}
			s.regionaltargettcpproxies = &cloud.MockRegionTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
			}
			s.regionalbackendservices = &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			}
			s.regionalforwardingrules = &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			}
			s.regionalhealthchecks = &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionHealthChecksObj{},
			}
			s.instancegroups = &cloud.MockInstanceGroups{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockInstanceGroupsObj{
					*meta.ZonalKey("my-cluster-apiserver-us-central1-a", "us-central1-a"): {
						Obj: &compute.InstanceGroup{
							Name:     "my-cluster-apiserver-us-central1-a",
							SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-apiserver-us-central1-a",
						},
					},
				},
			}

			// Run reconciliation
			err = s.Reconcile(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify regional external LB resources
			if tt.wantRegionalTargetTCPProxy {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := s.regionaltargettcpproxies.(*cloud.MockRegionTargetTcpProxies).Objects[*key]; !exists {
					t.Errorf("Expected regional target TCP proxy to be created")
				}
			}

			if tt.wantRegionalAddress {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := s.addresses.(*cloud.MockAddresses).Objects[*key]; !exists {
					t.Errorf("Expected regional address to be created")
				}
			}

			if tt.wantRegionalBackendService {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := s.regionalbackendservices.(*cloud.MockRegionBackendServices).Objects[*key]; !exists {
					t.Errorf("Expected regional backend service to be created")
				}
			}

			if tt.wantRegionalForwardingRule {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := s.regionalforwardingrules.(*cloud.MockForwardingRules).Objects[*key]; !exists {
					t.Errorf("Expected regional forwarding rule to be created")
				}
			}

			if tt.wantRegionalHealthCheck {
				key := meta.RegionalKey("my-cluster-apiserver", "us-central1")
				if _, exists := s.regionalhealthchecks.(*cloud.MockRegionHealthChecks).Objects[*key]; !exists {
					t.Errorf("Expected regional health check to be created")
				}
			}
		})
	}
}

// TestService_Delete_RegionalExternal tests the full deletion flow
// for RegionalExternal load balancer type.
func TestService_Delete_RegionalExternal(t *testing.T) {
	tests := []struct {
		name                      string
		lbType                    infrav1.LoadBalancerType
		setupRegionalResources    bool
		wantRegionalResourcesGone bool
		wantErr                   bool
	}{
		{
			name:                      "RegionalExternal deletes all regional resources",
			lbType:                    infrav1.RegionalExternal,
			setupRegionalResources:    true,
			wantRegionalResourcesGone: true,
			wantErr:                   false,
		},
		{
			name:                      "No resources exist - deletion succeeds (no-op)",
			lbType:                    infrav1.RegionalExternal,
			setupRegionalResources:    false,
			wantRegionalResourcesGone: true,
			wantErr:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			clusterScope, err := getBaseClusterScope()
			if err != nil {
				t.Fatal(err)
			}

			// Set the load balancer type
			clusterScope.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(tt.lbType),
			}

			// Initialize mocks
			s := New(clusterScope)
			key := meta.RegionalKey("my-cluster-apiserver", "us-central1")

			// Setup initial resources if requested
			if tt.setupRegionalResources {
				s.regionalforwardingrules = &cloud.MockForwardingRules{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects: map[meta.Key]*cloud.MockForwardingRulesObj{
						*key: {
							Obj: &compute.ForwardingRule{
								Name:   "my-cluster-apiserver",
								Region: "us-central1",
							},
						},
					},
				}
				s.regionaltargettcpproxies = &cloud.MockRegionTargetTcpProxies{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects: map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{
						*key: {
							Obj: &compute.TargetTcpProxy{
								Name:   "my-cluster-apiserver",
								Region: "us-central1",
							},
						},
					},
				}
				s.addresses = &cloud.MockAddresses{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects: map[meta.Key]*cloud.MockAddressesObj{
						*key: {
							Obj: &compute.Address{
								Name:   "my-cluster-apiserver",
								Region: "us-central1",
							},
						},
					},
				}
				s.regionalbackendservices = &cloud.MockRegionBackendServices{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects: map[meta.Key]*cloud.MockRegionBackendServicesObj{
						*key: {
							Obj: &compute.BackendService{
								Name:   "my-cluster-apiserver",
								Region: "us-central1",
							},
						},
					},
				}
				s.regionalhealthchecks = &cloud.MockRegionHealthChecks{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects: map[meta.Key]*cloud.MockRegionHealthChecksObj{
						*key: {
							Obj: &compute.HealthCheck{
								Name:   "my-cluster-apiserver",
								Region: "us-central1",
							},
						},
					},
				}
			} else {
				// Empty mocks
				s.regionalforwardingrules = &cloud.MockForwardingRules{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
				}
				s.regionaltargettcpproxies = &cloud.MockRegionTargetTcpProxies{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects:       map[meta.Key]*cloud.MockRegionTargetTcpProxiesObj{},
				}
				s.addresses = &cloud.MockAddresses{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects:       map[meta.Key]*cloud.MockAddressesObj{},
				}
				s.regionalbackendservices = &cloud.MockRegionBackendServices{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
				}
				s.regionalhealthchecks = &cloud.MockRegionHealthChecks{
					ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
					Objects:       map[meta.Key]*cloud.MockRegionHealthChecksObj{},
				}
			}

			// Run deletion
			err = s.Delete(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify all regional resources are deleted
			if tt.wantRegionalResourcesGone {
				if _, exists := s.regionalforwardingrules.(*cloud.MockForwardingRules).Objects[*key]; exists {
					t.Errorf("Regional forwarding rule still exists after deletion")
				}
				if _, exists := s.regionaltargettcpproxies.(*cloud.MockRegionTargetTcpProxies).Objects[*key]; exists {
					t.Errorf("Regional target TCP proxy still exists after deletion")
				}
				if _, exists := s.addresses.(*cloud.MockAddresses).Objects[*key]; exists {
					t.Errorf("Regional address still exists after deletion")
				}
				if _, exists := s.regionalbackendservices.(*cloud.MockRegionBackendServices).Objects[*key]; exists {
					t.Errorf("Regional backend service still exists after deletion")
				}
				if _, exists := s.regionalhealthchecks.(*cloud.MockRegionHealthChecks).Objects[*key]; exists {
					t.Errorf("Regional health check still exists after deletion")
				}
			}
		})
	}
}

// TestService_Reconcile_BackwardCompatibility ensures that existing LB types
// route correctly without attempting to create regional external resources.
// This test validates the routing logic only, not full reconciliation.
func TestService_Reconcile_BackwardCompatibility(t *testing.T) {
	tests := []struct {
		name                   string
		lbType                 *infrav1.LoadBalancerType
		wantIsRegionalExternal bool
	}{
		{
			name:                   "Default (nil) routes to global path",
			lbType:                 nil,
			wantIsRegionalExternal: false,
		},
		{
			name:                   "External routes to global path",
			lbType:                 ptr.To(infrav1.External),
			wantIsRegionalExternal: false,
		},
		{
			name:                   "InternalExternal routes to global path",
			lbType:                 ptr.To(infrav1.InternalExternal),
			wantIsRegionalExternal: false,
		},
		{
			name:                   "Internal routes to global path (no external LB)",
			lbType:                 ptr.To(infrav1.Internal),
			wantIsRegionalExternal: false,
		},
		{
			name:                   "RegionalExternal routes to regional path",
			lbType:                 ptr.To(infrav1.RegionalExternal),
			wantIsRegionalExternal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test the routing logic - don't actually run reconciliation
			lbType := ptr.Deref(tt.lbType, infrav1.External)
			isRegional := isRegionalExternalLoadBalancer(lbType)

			if isRegional != tt.wantIsRegionalExternal {
				t.Errorf("isRegionalExternalLoadBalancer() = %v, want %v", isRegional, tt.wantIsRegionalExternal)
			}

			// Verify the routing matches expectations
			if tt.wantIsRegionalExternal && !isRegional {
				t.Error("Expected to route to regional path but would route to global")
			}
			if !tt.wantIsRegionalExternal && isRegional {
				t.Error("Expected to route to global path but would route to regional")
			}
		})
	}
}
