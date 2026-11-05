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
	"testing"

	. "github.com/onsi/gomega"
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
