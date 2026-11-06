/*
 Copyright 2026 The Kubernetes Authors.

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

package scope

import (
	"context"
	"testing"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/shared"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestClusterScope_HealthCheckSpec(t *testing.T) {
	tests := []struct {
		name        string
		networkSpec infrav1.NetworkSpec
		wantedPort  int64
	}{
		{
			name:        "default value",
			networkSpec: infrav1.NetworkSpec{},
			wantedPort:  defaultHealthCheckPort,
		},
		{
			name: "given value",
			networkSpec: infrav1.NetworkSpec{
				LoadBalancerBackendPort: new(int32(1234)),
			},
			wantedPort: int64(1234),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ClusterScope{
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
				GCPCluster: &infrav1.GCPCluster{
					Spec: infrav1.GCPClusterSpec{
						Network: tt.networkSpec,
					},
				},
			}
			g := NewWithT(t)
			g.Expect(s.HealthCheckSpec("my-lb").HttpsHealthCheck.Port).To(Equal(tt.wantedPort))
		})
	}
}

// fakeTagClient satisfies both shared.TagValueGetter and shared.TagBinder and counts Close calls.
type fakeTagClient struct {
	closes *int
}

func (f fakeTagClient) GetNamespacedTagValue(_ context.Context, _ *rmpb.GetNamespacedTagValueRequest, _ ...gax.CallOption) (*rmpb.TagValue, error) {
	return &rmpb.TagValue{Name: "tag123", Parent: "foo"}, nil
}

func (f fakeTagClient) CreateTagBinding(_ context.Context, _ *rmpb.CreateTagBindingRequest, _ ...gax.CallOption) (*resourcemanager.CreateTagBindingOperation, error) {
	return nil, nil
}

func (f fakeTagClient) Close() error {
	*f.closes++
	return nil
}

func TestClusterScope_CloseClients(t *testing.T) {
	tags := infrav1.ResourceManagerTags{
		{ParentID: "foo", Key: "key1", Value: "value1"},
	}

	tests := []struct {
		name       string
		tagManager func(g *WithT, closes *int) *shared.TagManager
		wantCloses int
	}{
		{
			name:       "no tag manager",
			tagManager: func(_ *WithT, _ *int) *shared.TagManager { return nil },
		},
		{
			name: "tag manager whose clients were never built",
			tagManager: func(_ *WithT, closes *int) *shared.TagManager {
				return shared.NewLazyTagManager(
					func(_ context.Context) (shared.TagValueGetter, error) { return fakeTagClient{closes: closes}, nil },
					func(_ context.Context) (shared.TagBinder, error) { return fakeTagClient{closes: closes}, nil },
				)
			},
		},
		{
			name: "tag manager with a built client closes it once, however often CloseClients is called",
			tagManager: func(g *WithT, closes *int) *shared.TagManager {
				m := shared.NewLazyTagManager(
					func(_ context.Context) (shared.TagValueGetter, error) { return fakeTagClient{closes: closes}, nil },
					func(_ context.Context) (shared.TagBinder, error) { return fakeTagClient{closes: closes}, nil },
				)
				g.Expect(m.TagInstance(t.Context(), &compute.Instance{}, tags)).To(Succeed())
				return m
			},
			wantCloses: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			closes := 0
			// A bare scope has no patch helper, so this also shows CloseClients never tries to patch the cluster.
			s := &ClusterScope{GCPServices: GCPServices{TagManager: tt.tagManager(g, &closes)}}

			g.Expect(s.CloseClients()).To(Succeed())
			g.Expect(s.CloseClients()).To(Succeed())
			g.Expect(closes).To(Equal(tt.wantCloses))
		})
	}
}
