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

package firewalls

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/pkg/errors"
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
	Status: infrav1.GCPClusterStatus{
		Network: infrav1.Network{
			FirewallRules: map[string]string{
				fmt.Sprintf("allow-%s-healthchecks", "my-cluster"): "test",
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
	Status: infrav1.GCPClusterStatus{
		Network: infrav1.Network{
			FirewallRules: map[string]string{
				"my-cluster-apiserver":    "test",
				"my-cluster-apiintserver": "test",
			},
		},
	},
}

type testCase struct {
	name          string
	scope         func() Scope
	mockFirewalls *cloud.MockFirewalls
	wantErr       bool
	assert        func(ctx context.Context, t testCase) error
}

func TestService_Reconcile(t *testing.T) {
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
			name:  "firewall rule does not exist successful create",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockFirewallsObj{},
			},
			assert: func(ctx context.Context, t testCase) error {
				key := meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name))
				fwRule, err := t.mockFirewalls.Get(ctx, key)
				if err != nil {
					return err
				}

				if _, ok := fakeGCPCluster.Status.Network.FirewallRules[fwRule.Name]; !ok {
					return errors.New("firewall rule was created but with wrong values")
				}
				return nil
			},
		},
		{
			name:  "firewall rule already exist (should return existing firewall rule)",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockFirewallsObj{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name)): {},
				},
			},
		},
		{
			name:  "error getting instance with non 404 error code (should return an error)",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockFirewallsObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockFirewalls, _ ...cloud.Option) (bool, *compute.Firewall, error) {
					return true, &compute.Firewall{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "firewall rule creation fails (should return an error)",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockFirewallsObj{},
				InsertError: map[meta.Key]error{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name)): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
		{
			name:  "firewall return no error using shared vpc",
			scope: func() Scope { return clusterScopeSharedVpc },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockFirewallsObj{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name)): {},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.firewalls = tt.mockFirewalls
			err := s.Reconcile(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.assert != nil {
				err = tt.assert(ctx, tt)
				if err != nil {
					t.Errorf("firewall rule was not created as expected: %v", err)
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
			name:  "firewall rule does not exist, should do nothing",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				DeleteError: map[meta.Key]error{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name)): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
		},
		{
			name:  "error deleting firewall rule, should return error",
			scope: func() Scope { return clusterScope },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				DeleteError: map[meta.Key]error{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", fakeGCPCluster.ObjectMeta.Name)): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
		{
			name:  "firewall rule deletion with shared vpc",
			scope: func() Scope { return clusterScopeSharedVpc },
			mockFirewalls: &cloud.MockFirewalls{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				DeleteError: map[meta.Key]error{
					*meta.GlobalKey(fmt.Sprintf("allow-%s-healthchecks", *fakeGCPCluster.Spec.Network.Name)): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.firewalls = tt.mockFirewalls
			err := s.Delete(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
