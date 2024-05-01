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
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var lbTypeInternal = infrav1.Internal

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1.AddToScheme(scheme.Scheme)
}

func getBaseClusterScope() (*scope.ClusterScope, error) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	fakeCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{},
	}

	fakeGCPCluster := &infrav1.GCPCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
		Spec: infrav1.GCPClusterSpec{
			Project: "my-proj",
			Region:  "us-central1",
			Network: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					infrav1.SubnetSpec{
						Name:      "control-plane",
						CidrBlock: "10.0.0.1/28",
						Region:    "us-central1",
						Purpose:   ptr.To[string]("INTERNAL_HTTPS_LOAD_BALANCER"),
					},
				},
			},
		},
		Status: infrav1.GCPClusterStatus{
			FailureDomains: clusterv1.FailureDomains{
				"us-central1-a": clusterv1.FailureDomainSpec{ControlPlane: true},
			},
		},
	}
	clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		return nil, err
	}

	return clusterScope, nil
}

func TestService_createOrGetInstanceGroup(t *testing.T) {
	tests := []struct {
		name              string
		scope             func(s *scope.ClusterScope) Scope
		mockInstanceGroup *cloud.MockInstanceGroups
		want              []*compute.InstanceGroup
		wantErr           bool
	}{
		{
			name:  "error getting instanceGroup with non 404 error code (should return an error)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockInstanceGroup: &cloud.MockInstanceGroups{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstanceGroupsObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockInstanceGroups, _ ...cloud.Option) (bool, *compute.InstanceGroup, error) {
					return true, &compute.InstanceGroup{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			want:    []*compute.InstanceGroup{},
			wantErr: true,
		},
		{
			name: "instanceGroup name is overridden (should create instanceGroup)",
			scope: func(s *scope.ClusterScope) Scope {
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					APIServerInstanceGroupTagOverride: ptr.To[string]("master"),
				}
				return s
			},
			mockInstanceGroup: &cloud.MockInstanceGroups{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstanceGroupsObj{},
			},
			want: []*compute.InstanceGroup{
				{
					Name:       "my-cluster-master-us-central1-a",
					NamedPorts: []*compute.NamedPort{{Name: "apiserver", Port: 6443}},
					SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-master-us-central1-a",
				},
			},
		},
		{
			name:  "instanceGroup does not exist (should create instanceGroup)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			mockInstanceGroup: &cloud.MockInstanceGroups{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstanceGroupsObj{},
			},
			want: []*compute.InstanceGroup{
				{
					Name:       "my-cluster-apiserver-us-central1-a",
					NamedPorts: []*compute.NamedPort{{Name: "apiserver", Port: 6443}},
					SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-apiserver-us-central1-a",
				},
			},
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
			s.instancegroups = tt.mockInstanceGroup
			got, err := s.createOrGetInstanceGroups(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetInstanceGroups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetInstanceGroups() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetHealthCheck(t *testing.T) {
	tests := []struct {
		name             string
		scope            func(s *scope.ClusterScope) Scope
		lbName           string
		mockHealthChecks *cloud.MockHealthChecks
		want             *compute.HealthCheck
		wantErr          bool
	}{
		{
			name:   "health check does not exist for external load balancer (should create healthcheck)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbName: infrav1.APIServerRoleTagValue,
			mockHealthChecks: &cloud.MockHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockHealthChecksObj{},
			},
			want: &compute.HealthCheck{
				CheckIntervalSec:   10,
				HealthyThreshold:   5,
				HttpsHealthCheck:   &compute.HTTPSHealthCheck{Port: 6443, PortSpecification: "USE_FIXED_PORT", RequestPath: "/readyz"},
				Name:               "my-cluster-apiserver",
				SelfLink:           "https://www.googleapis.com/compute/v1/projects/proj-id/global/healthChecks/my-cluster-apiserver",
				TimeoutSec:         5,
				Type:               "HTTPS",
				UnhealthyThreshold: 3,
			},
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
			s.healthchecks = tt.mockHealthChecks
			got, err := s.createOrGetHealthCheck(ctx, tt.lbName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetHealthChecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetHealthCheck() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalHealthCheck(t *testing.T) {
	tests := []struct {
		name             string
		scope            func(s *scope.ClusterScope) Scope
		lbName           string
		mockHealthChecks *cloud.MockRegionHealthChecks
		want             *compute.HealthCheck
		wantErr          bool
	}{
		{
			name: "regional health check does not exist for internal load balancer (should create healthcheck)",
			scope: func(s *scope.ClusterScope) Scope {
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					LoadBalancerType: &lbTypeInternal,
				}
				return s
			},
			lbName: infrav1.InternalRoleTagValue,
			mockHealthChecks: &cloud.MockRegionHealthChecks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionHealthChecksObj{},
			},
			want: &compute.HealthCheck{
				CheckIntervalSec:   10,
				HealthyThreshold:   5,
				HttpsHealthCheck:   &compute.HTTPSHealthCheck{Port: 6443, PortSpecification: "USE_FIXED_PORT", RequestPath: "/readyz"},
				Name:               "my-cluster-api-internal",
				Region:             "us-central1",
				SelfLink:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-api-internal",
				TimeoutSec:         5,
				Type:               "HTTPS",
				UnhealthyThreshold: 3,
			},
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
			s.regionalhealthchecks = tt.mockHealthChecks
			got, err := s.createOrGetRegionalHealthCheck(ctx, tt.lbName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetRegionalHealthChecks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrRegionalGetHealthCheck() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetBackendService(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbName             string
		healthCheck        *compute.HealthCheck
		instanceGroups     []*compute.InstanceGroup
		mockBackendService *cloud.MockBackendServices
		want               *compute.BackendService
		wantErr            bool
	}{
		{
			name:   "backend service does not exist for external load balancer (should create backendservice)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbName: infrav1.APIServerRoleTagValue,
			healthCheck: &compute.HealthCheck{
				HttpsHealthCheck: &compute.HTTPSHealthCheck{Port: 6443, PortSpecification: "USE_FIXED_PORT", RequestPath: "/readyz"},
				Name:             "my-cluster-apiserver",
				SelfLink:         "https://www.googleapis.com/compute/v1/projects/proj-id/global/healthChecks/my-cluster-apiserver",
			},
			instanceGroups: []*compute.InstanceGroup{
				{
					Name:       "my-cluster-master-us-central1-a",
					NamedPorts: []*compute.NamedPort{{Name: "apiserver", Port: 6443}},
					SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-master-us-central1-a",
				},
			},
			mockBackendService: &cloud.MockBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockBackendServicesObj{},
			},
			want: &compute.BackendService{
				Backends: []*compute.Backend{
					{
						BalancingMode: "UTILIZATION",
						Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-master-us-central1-a",
					},
				},
				HealthChecks: []string{
					"https://www.googleapis.com/compute/v1/projects/proj-id/global/healthChecks/my-cluster-apiserver",
				},
				LoadBalancingScheme: "EXTERNAL",
				Name:                "my-cluster-apiserver",
				PortName:            "apiserver",
				Protocol:            "TCP",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/global/backendServices/my-cluster-apiserver",
				TimeoutSec:          600,
			},
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
			s.backendservices = tt.mockBackendService
			mode := loadBalancingModeUtilization
			got, err := s.createOrGetBackendService(ctx, tt.lbName, mode, tt.instanceGroups, tt.healthCheck)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetBackendService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetBackendService() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalBackendService(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbName             string
		healthCheck        *compute.HealthCheck
		instanceGroups     []*compute.InstanceGroup
		mockBackendService *cloud.MockRegionBackendServices
		want               *compute.BackendService
		wantErr            bool
	}{
		{
			name: "regional backend service does not exist for internal load balancer (should create regional backendservice)",
			scope: func(s *scope.ClusterScope) Scope {
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					LoadBalancerType: &lbTypeInternal,
				}
				s.GCPCluster.Status.Network.SelfLink = ptr.To[string]("https://www.googleapis.com/compute/v1/projects/openshift-dev-installer/global/networks/bfournie-capg-test-5jp2d-network")
				return s
			},
			lbName: infrav1.InternalRoleTagValue,
			healthCheck: &compute.HealthCheck{
				HttpsHealthCheck: &compute.HTTPSHealthCheck{Port: 6443, PortSpecification: "USE_FIXED_PORT", RequestPath: "/readyz"},
				Name:             "my-cluster-api-internal",
				Region:           "us-central1",
				SelfLink:         "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-api-internal",
			},
			instanceGroups: []*compute.InstanceGroup{
				{
					Name:       "my-cluster-apiserver-us-central1-a",
					NamedPorts: []*compute.NamedPort{{Name: "apiserver", Port: 6443}},
					SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-master-us-central1-a",
				},
			},
			mockBackendService: &cloud.MockRegionBackendServices{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockRegionBackendServicesObj{},
			},
			want: &compute.BackendService{
				Backends: []*compute.Backend{
					{
						BalancingMode: "CONNECTION",
						Group:         "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instanceGroups/my-cluster-master-us-central1-a",
					},
				},
				HealthChecks: []string{
					"https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/healthChecks/my-cluster-api-internal",
				},
				LoadBalancingScheme: "INTERNAL",
				Name:                "my-cluster-api-internal",
				Network:             "https://www.googleapis.com/compute/v1/projects/openshift-dev-installer/global/networks/bfournie-capg-test-5jp2d-network",
				PortName:            "",
				Protocol:            "TCP",
				Region:              "us-central1",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/backendServices/my-cluster-api-internal",
				TimeoutSec:          600,
			},
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
			got, err := s.createOrGetRegionalBackendService(ctx, tt.lbName, tt.instanceGroups, tt.healthCheck)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetRegionalBackendService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetRegionalBackendService() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetAddress(t *testing.T) {
	tests := []struct {
		name        string
		scope       func(s *scope.ClusterScope) Scope
		lbName      string
		mockAddress *cloud.MockGlobalAddresses
		want        *compute.Address
		wantErr     bool
	}{
		{
			name:   "address does not exist for external load balancer (should create address)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbName: infrav1.APIServerRoleTagValue,
			mockAddress: &cloud.MockGlobalAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockGlobalAddressesObj{},
			},
			want: &compute.Address{
				IpVersion:   "IPV4",
				Name:        "my-cluster-apiserver",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/global/addresses/my-cluster-apiserver",
				AddressType: "EXTERNAL",
			},
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
			s.addresses = tt.mockAddress
			got, err := s.createOrGetAddress(ctx, tt.lbName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetAddress() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetInternalAddress(t *testing.T) {
	tests := []struct {
		name            string
		scope           func(s *scope.ClusterScope) Scope
		lbName          string
		mockAddress     *cloud.MockAddresses
		mockSubnetworks *cloud.MockSubnetworks
		want            *compute.Address
		wantErr         bool
	}{
		{
			name: "address does not exist for internal load balancer (should create address)",
			scope: func(s *scope.ClusterScope) Scope {
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					LoadBalancerType: &lbTypeInternal,
				}
				return s
			},
			lbName: infrav1.InternalRoleTagValue,
			mockAddress: &cloud.MockAddresses{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockAddressesObj{},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey("control-plane", "us-central1"): {},
				},
			},
			want: &compute.Address{
				IpVersion:   "IPV4",
				Name:        "my-cluster-api-internal",
				Region:      "us-central1",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-api-internal",
				AddressType: "INTERNAL",
				Purpose:     "GCE_ENDPOINT",
			},
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
			s.internaladdresses = tt.mockAddress
			s.subnets = tt.mockSubnetworks
			got, err := s.createOrGetInternalAddress(ctx, tt.lbName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetInternalAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetInternalAddress() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetTargetTCPProxy(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		backendService     *compute.BackendService
		mockTargetTCPProxy *cloud.MockTargetTcpProxies
		want               *compute.TargetTcpProxy
		wantErr            bool
	}{
		{
			name:  "target tcp proxy does not exist for external load balancer (should create target tp proxy)",
			scope: func(s *scope.ClusterScope) Scope { return s },
			backendService: &compute.BackendService{
				Name: "my-cluster-api-internal",
			},
			mockTargetTCPProxy: &cloud.MockTargetTcpProxies{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockTargetTcpProxiesObj{},
			},
			want: &compute.TargetTcpProxy{
				Name:        "my-cluster-apiserver",
				ProxyHeader: "NONE",
				SelfLink:    "https://www.googleapis.com/compute/v1/projects/proj-id/global/targetTcpProxies/my-cluster-apiserver",
			},
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
			s.targettcpproxies = tt.mockTargetTCPProxy
			got, err := s.createOrGetTargetTCPProxy(ctx, tt.backendService)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetTargetTCPProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service s.createOrGetTargetTCPProxy() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetForwardingRule(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbName             string
		backendService     *compute.BackendService
		targetTcpproxy     *compute.TargetTcpProxy
		address            *compute.Address
		mockForwardingRule *cloud.MockGlobalForwardingRules
		want               *compute.ForwardingRule
		wantErr            bool
	}{
		{
			name:   "forwarding rule does not exist for external load balancer (should create forwardingrule)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbName: infrav1.APIServerRoleTagValue,
			address: &compute.Address{
				Name:     "my-cluster-apiserver",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
			},
			backendService: &compute.BackendService{},
			targetTcpproxy: &compute.TargetTcpProxy{
				Name: "my-cluster-apiserver",
			},
			mockForwardingRule: &cloud.MockGlobalForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockGlobalForwardingRulesObj{},
			},
			want: &compute.ForwardingRule{
				IPAddress:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-apiserver",
				IPProtocol:          "TCP",
				LoadBalancingScheme: "EXTERNAL",
				PortRange:           "443-443",
				Name:                "my-cluster-apiserver",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/global/forwardingRules/my-cluster-apiserver",
			},
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
			s.forwardingrules = tt.mockForwardingRule
			var fwdRule *compute.ForwardingRule
			fwdRule, err = s.createOrGetForwardingRule(ctx, tt.lbName, tt.targetTcpproxy, tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetForwardingRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, fwdRule); d != "" {
				t.Errorf("Service s.createOrGetForwardingRule() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestService_createOrGetRegionalForwardingRule(t *testing.T) {
	tests := []struct {
		name               string
		scope              func(s *scope.ClusterScope) Scope
		lbName             string
		backendService     *compute.BackendService
		targetTcpproxy     *compute.TargetTcpProxy
		address            *compute.Address
		mockSubnetworks    *cloud.MockSubnetworks
		mockForwardingRule *cloud.MockForwardingRules
		want               *compute.ForwardingRule
		wantErr            bool
	}{
		{
			name:   "regional forwarding rule does not exist for internal load balancer (should create forwardingrule)",
			scope:  func(s *scope.ClusterScope) Scope { return s },
			lbName: infrav1.InternalRoleTagValue,
			address: &compute.Address{
				Name:     "my-cluster-api-internal",
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-api-internal",
			},
			backendService: &compute.BackendService{
				Name: "my-cluster-api-internal",
			},
			targetTcpproxy: &compute.TargetTcpProxy{},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey("control-plane", "us-central1"): {},
				},
			},
			mockForwardingRule: &cloud.MockForwardingRules{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockForwardingRulesObj{},
			},
			want: &compute.ForwardingRule{
				IPAddress:           "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/addresses/my-cluster-api-internal",
				IPProtocol:          "TCP",
				LoadBalancingScheme: "INTERNAL",
				Ports:               []string{"6443", "22623"},
				Region:              "us-central1",
				Name:                "my-cluster-api-internal",
				SelfLink:            "https://www.googleapis.com/compute/v1/projects/proj-id/regions/us-central1/forwardingRules/my-cluster-api-internal",
			},
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
			var fwdRule *compute.ForwardingRule
			s.subnets = tt.mockSubnetworks
			fwdRule, err = s.createOrGetRegionalForwardingRule(ctx, tt.lbName, tt.backendService, tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service s.createOrGetRegionalForwardingRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(tt.want, fwdRule); d != "" {
				t.Errorf("Service s.createOrGetRegionalForwardingRule() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
