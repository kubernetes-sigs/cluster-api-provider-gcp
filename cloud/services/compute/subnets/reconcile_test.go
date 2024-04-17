/*
Copyright 2022 The Kubernetes Authors.

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

package subnets

import (
	"context"
	"errors"
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
	name            string
	scope           func() Scope
	mockSubnetworks *cloud.MockSubnetworks
	wantErr         bool
	assert          func(ctx context.Context, t testCase) error
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

	tests := []testCase{
		{
			name:  "subnet already exist (should return existing subnet)",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey(fakeGCPCluster.Spec.Network.Subnets[0].Name, fakeGCPCluster.Spec.Region): {},
				},
			},
		},
		{
			name:  "error getting instance with non 404 error code (should return an error)",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockSubnetworks) (bool, *compute.Subnetwork, error) {
					return true, &compute.Subnetwork{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "subnet does not exist (should create subnet)",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
			},
			assert: func(ctx context.Context, t testCase) error {
				key := meta.RegionalKey(fakeGCPCluster.Spec.Network.Subnets[0].Name, fakeGCPCluster.Spec.Region)
				subnet, err := t.mockSubnetworks.Get(ctx, key)
				if err != nil {
					return err
				}

				if subnet.Name != fakeGCPCluster.Spec.Network.Subnets[0].Name ||
					subnet.IpCidrRange != fakeGCPCluster.Spec.Network.Subnets[0].CidrBlock ||
					subnet.Purpose != *fakeGCPCluster.Spec.Network.Subnets[0].Purpose {
					return errors.New("subnet was created but with wrong values")
				}

				return nil
			},
		},
		{
			name:  "subnet creation fails (should return an error)",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
				InsertError: map[meta.Key]error{
					*meta.RegionalKey(fakeGCPCluster.Spec.Network.Subnets[0].Name, fakeGCPCluster.Spec.Region): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.subnets = tt.mockSubnetworks
			err := s.Reconcile(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.assert != nil {
				err = tt.assert(ctx, tt)
				if err != nil {
					t.Errorf("subnet was not created as expected: %v", err)
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

	tests := []testCase{
		{
			name:  "subnet does not exist, should do nothing",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(fakeGCPCluster.Spec.Network.Subnets[0].Name, fakeGCPCluster.Spec.Region): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
		},
		{
			name:  "error deleting subnet, should return error",
			scope: func() Scope { return clusterScope },
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(fakeGCPCluster.Spec.Network.Subnets[0].Name, fakeGCPCluster.Spec.Region): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.subnets = tt.mockSubnetworks
			err := s.Delete(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
