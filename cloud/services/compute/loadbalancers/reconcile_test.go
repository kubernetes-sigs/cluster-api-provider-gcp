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
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
				tagOverride := "master"
				s.GCPCluster.Spec.LoadBalancer = infrav1.LoadBalancerSpec{
					APIServerInstanceGroupTagOverride: &tagOverride,
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
