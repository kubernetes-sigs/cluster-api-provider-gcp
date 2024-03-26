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

package networks

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
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

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1.AddToScheme(scheme.Scheme)
}

var fakeCluster = &clusterv1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: clusterv1.ClusterSpec{},
}

var fakeGCPCluster = &infrav1.GCPCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: infrav1.GCPClusterSpec{
		Project: "my-proj",
		Region:  "us-central1",
		Network: infrav1.NetworkSpec{
			Name: ptr.To("my-network"),
			Subnets: infrav1.Subnets{
				infrav1.SubnetSpec{
					Name:      "workers",
					CidrBlock: "10.0.0.1/28",
					Region:    "us-central1",
					Purpose:   ptr.To[string]("INTERNAL_HTTPS_LOAD_BALANCER"),
				},
			},
		},
	},
}

var fakeGCPClusterSharedVPC = &infrav1.GCPCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: infrav1.GCPClusterSpec{
		Project: "my-proj",
		Region:  "us-central1",
		Network: infrav1.NetworkSpec{
			HostProject: ptr.To("my-shared-vpc-project"),
			Name:        ptr.To("my-network"),
			Subnets: infrav1.Subnets{
				infrav1.SubnetSpec{
					Name:      "workers",
					CidrBlock: "10.0.0.1/28",
					Region:    "us-central1",
					Purpose:   ptr.To[string]("INTERNAL_HTTPS_LOAD_BALANCER"),
				},
			},
		},
	},
}

type testCase struct {
	name        string
	scope       func() Scope
	mockNetwork *cloud.MockNetworks
	mockRouter  *cloud.MockRouters
	wantErr     bool
	assert      func(ctx context.Context, t testCase) error
}

func TestService_createOrGetNetwork(t *testing.T) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clusterScopeSharedVpc, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPClusterSharedVPC,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []testCase{
		{
			name:  "network already exist (should return existing network)",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
			},
		},
		{
			name:  "error getting network instance with non 404 error code (should return an error)",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockNetworks, _ ...cloud.Option) (bool, *compute.Network, error) {
					return true, &compute.Network{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "network list error find issue shared vpc",
			scope: func() Scope { return clusterScopeSharedVpc },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockNetworks, _ ...cloud.Option) (bool, *compute.Network, error) {
					return true, &compute.Network{}, &googleapi.Error{Code: http.StatusNotFound}
				},
			},
			wantErr: true,
		},
		{
			name:  "network creation fails (should return an error)",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockNetworksObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockNetworks, _ ...cloud.Option) (bool, *compute.Network, error) {
					return true, &compute.Network{}, &googleapi.Error{Code: http.StatusNotFound}
				},
				InsertError: map[meta.Key]error{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.networks = tt.mockNetwork
			_, err := s.createOrGetNetwork(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetNetwork error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.assert != nil {
				err = tt.assert(ctx, tt)
				if err != nil {
					t.Errorf("network was not created as expected: %v", err)
					return
				}
			}
		})
	}
}

func TestService_createOrGetRouter(t *testing.T) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clusterScopeSharedVpc, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPClusterSharedVPC,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []testCase{
		{
			name:  "error getting router instance with non 404 error code (should return an error)",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
			},
			mockRouter: &cloud.MockRouters{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockRoutersObj{
					*meta.RegionalKey(fmt.Sprintf("%s-%s", *fakeGCPCluster.Spec.Network.Name, "router"), fakeGCPCluster.Spec.Region): {},
				},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockRouters, _ ...cloud.Option) (bool, *compute.Router, error) {
					return true, &compute.Router{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "router list error find issue shared vpc",
			scope: func() Scope { return clusterScopeSharedVpc },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
			},
			mockRouter: &cloud.MockRouters{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockRoutersObj{
					*meta.RegionalKey(fmt.Sprintf("%s-%s", *fakeGCPCluster.Spec.Network.Name, "router"), fakeGCPCluster.Spec.Region): {},
				},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockRouters, _ ...cloud.Option) (bool, *compute.Router, error) {
					return true, &compute.Router{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "router creation fails (should return an error)",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockNetworksObj{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): {},
				},
			},
			mockRouter: &cloud.MockRouters{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockRoutersObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockRouters, _ ...cloud.Option) (bool, *compute.Router, error) {
					return true, &compute.Router{}, &googleapi.Error{Code: http.StatusNotFound}
				},
				InsertError: map[meta.Key]error{
					*meta.RegionalKey(fmt.Sprintf("%s-%s", *fakeGCPCluster.Spec.Network.Name, "router"), fakeGCPCluster.Spec.Region): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.networks = tt.mockNetwork
			s.routers = tt.mockRouter

			network, err := s.createOrGetNetwork(ctx)
			if err != nil {
				t.Errorf("Service.createOrGetNetwork error = %v", err)
				return
			}

			_, err = s.createOrGetRouter(ctx, network)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetRouter error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.assert != nil {
				err = tt.assert(ctx, tt)
				if err != nil {
					t.Errorf("router was not created as expected: %v", err)
					return
				}
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clusterScopeSharedVpc, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPClusterSharedVPC,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []testCase{
		{
			name:  "network does not exist, should do nothing",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				GetError: map[meta.Key]error{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
		},
		{
			name:  "error deleting network, should return error",
			scope: func() Scope { return clusterScope },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				GetError: map[meta.Key]error{
					*meta.GlobalKey(*fakeGCPCluster.Spec.Network.Name): &googleapi.Error{Code: http.StatusBadGateway},
				},
			},
			wantErr: true,
		},
		{
			name:  "network shared vpc, should do nothing",
			scope: func() Scope { return clusterScopeSharedVpc },
			mockNetwork: &cloud.MockNetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.networks = tt.mockNetwork
			err := s.Delete(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
