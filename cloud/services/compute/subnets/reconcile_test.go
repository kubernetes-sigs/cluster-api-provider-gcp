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

package subnets

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	clusterName                  = "my-cluster"
	defaultSubnetName            = "workers"
	gcpProject                   = "my-proj"
	gcpRegion                    = "us-central1"
	hostProject                  = "my-shared-vpc-project"
	namespace                    = "default"
	nonCapiControlledDescription = "non-CAPG subnet"
	purpose                      = "INTERNAL_HTTPS_LOAD_BALANCER"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1.AddToScheme(scheme.Scheme)
}

type clusterSubnetTestConfig struct {
	Name   string
	Region string
}

func createNewScopeForSubnets(config []clusterSubnetTestConfig, sharedVpc bool) (*scope.ClusterScope, error) {
	subnets := make([]infrav1.SubnetSpec, 0, len(config))
	for i, testConfig := range config {
		subnet := infrav1.SubnetSpec{
			Name:      testConfig.Name,
			CidrBlock: generateCidrBlock(i),
			Region:    testConfig.Region,
			Purpose:   new(purpose),
		}
		subnets = append(subnets, subnet)
	}

	network := infrav1.NetworkSpec{
		Subnets: subnets,
	}
	if sharedVpc {
		network.HostProject = new(hostProject)
	}

	return scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
			},
			Spec: clusterv1.ClusterSpec{},
		},
		GCPCluster: &infrav1.GCPCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
			},
			Spec: infrav1.GCPClusterSpec{
				Project: gcpProject,
				Region:  gcpRegion,
				Network: network,
			},
		},
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
}

func generateCidrBlock(index int) string {
	return fmt.Sprintf("10.0.0.%d/28", index)
}

type testCase struct {
	name            string
	sharedVpc       bool
	clusterSubnets  []clusterSubnetTestConfig
	mockSubnetworks *cloud.MockSubnetworks
	wantErr         bool
	assert          func(ctx context.Context, t testCase) error
}

func TestService_Reconcile(t *testing.T) {
	tests := []testCase{
		{
			name: "subnet already exist (should return existing subnet)",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): {},
				},
			},
		},
		{
			name: "error getting instance with non 404 error code (should return an error)",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockSubnetworks, _ ...cloud.Option) (bool, *compute.Subnetwork, error) {
					return true, &compute.Subnetwork{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name: "subnet does not exist (should create subnet)",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "my-proj"},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
			},
			assert: func(ctx context.Context, t testCase) error {
				key := meta.RegionalKey(defaultSubnetName, gcpRegion)
				subnet, err := t.mockSubnetworks.Get(ctx, key)
				if err != nil {
					return err
				}

				if subnet.Name != t.clusterSubnets[0].Name ||
					subnet.IpCidrRange != generateCidrBlock(0) ||
					subnet.Purpose != purpose {
					return errors.New("subnet was created but with wrong values")
				}

				return nil
			},
		},
		{
			name: "subnet creation fails (should return an error)",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
				InsertError: map[meta.Key]error{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): &googleapi.Error{Code: http.StatusBadRequest},
				},
			},
			wantErr: true,
		},
		{
			name: "subnet list error find issue shared vpc",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				Objects:       map[meta.Key]*cloud.MockSubnetworksObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockSubnetworks, _ ...cloud.Option) (bool, *compute.Subnetwork, error) {
					return true, &compute.Subnetwork{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr:   true,
			sharedVpc: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			testScope, err := createNewScopeForSubnets(tt.clusterSubnets, tt.sharedVpc)
			if err != nil {
				t.Error(err)
				return
			}
			s := New(testScope)
			s.subnets = tt.mockSubnetworks
			err = s.Reconcile(ctx)
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
	tests := []testCase{
		{
			name: "subnet does not exist, should do nothing",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
		},
		{
			name: "error deleting subnet, should return error",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): &googleapi.Error{Code: http.StatusBadRequest},
				},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): {Obj: compute.Subnetwork{Description: infrav1.ClusterTagKey(clusterName)}},
				},
			},
			wantErr: true,
		},
		{
			name: "subnet not created by CAPI, should not try to delete it",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): &googleapi.Error{Code: http.StatusBadRequest},
				},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): {Obj: compute.Subnetwork{Description: nonCapiControlledDescription}},
				},
			},
			wantErr: false,
		},
		{
			name: "subnet deletion with shared vpc",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				DeleteError: map[meta.Key]error{
					*meta.RegionalKey(defaultSubnetName, gcpRegion): &googleapi.Error{Code: http.StatusNotFound},
				},
			},
			sharedVpc: true,
		},
		{
			name: "should delete all CAPI owned subnets (continuing past external subnets)",
			clusterSubnets: []clusterSubnetTestConfig{
				{
					Name:   defaultSubnetName,
					Region: gcpRegion,
				},
				{
					Name:   "my-new-subnet",
					Region: "us-central2",
				},
				{
					Name:   "my-new-subnet-1",
					Region: "us-central2",
				},
				{
					Name:   "my-new-subnet-2",
					Region: "us-central2",
				},
			},
			mockSubnetworks: &cloud.MockSubnetworks{
				ProjectRouter: &cloud.SingleProjectRouter{ID: gcpProject},
				Objects: map[meta.Key]*cloud.MockSubnetworksObj{
					*meta.RegionalKey(defaultSubnetName, gcpRegion):     {Obj: compute.Subnetwork{Description: nonCapiControlledDescription}},
					*meta.RegionalKey("my-new-subnet", "us-central2"):   {Obj: compute.Subnetwork{Description: infrav1.ClusterTagKey(clusterName)}},
					*meta.RegionalKey("my-new-subnet-1", "us-central2"): {Obj: compute.Subnetwork{Description: nonCapiControlledDescription}},
					*meta.RegionalKey("my-new-subnet-2", "us-central2"): {Obj: compute.Subnetwork{Description: infrav1.ClusterTagKey(clusterName)}},
				},
			},
			assert: func(_ context.Context, t testCase) error {
				desired := []meta.Key{
					*meta.RegionalKey(defaultSubnetName, gcpRegion),
					*meta.RegionalKey("my-new-subnet-1", "us-central2"),
				}

				actual := make([]meta.Key, 0, len(t.mockSubnetworks.Objects))
				for key := range t.mockSubnetworks.Objects {
					actual = append(actual, key)
				}

				if diff := cmp.Diff(desired, actual, cmpopts.SortSlices(func(a meta.Key, b meta.Key) int {
					return strings.Compare(a.String(), b.String())
				})); diff != "" {
					return fmt.Errorf("unexpected diff (-want +got):\n%s", diff)
				}

				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			testScope, err := createNewScopeForSubnets(tt.clusterSubnets, tt.sharedVpc)
			if err != nil {
				t.Error(err)
				return
			}
			s := New(testScope)
			s.subnets = tt.mockSubnetworks
			err = s.Delete(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.assert != nil {
				err = tt.assert(ctx, tt)
				if err != nil {
					t.Errorf("subnet was not deleted as expected: %v", err)
					return
				}
			}
		})
	}
}
