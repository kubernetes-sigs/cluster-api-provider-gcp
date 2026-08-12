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

package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clusterWithRule(name string, rule infrav1.FirewallRule) *infrav1.GCPCluster {
	return &infrav1.GCPCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: infrav1.GCPClusterSpec{
			Project: "test-project",
			Region:  "us-central1",
			Network: infrav1.NetworkSpec{
				Firewall: infrav1.FirewallSpec{
					FirewallRules: []infrav1.FirewallRule{rule},
				},
			},
		},
	}
}

// These specs cover the interaction between the omitempty json tag, the minimum of 1 and
// the default of 1000. The only value omitempty drops is 0, and 0 is the one value the
// minimum rejects, so nothing a user can express is lost: an omitted priority is stored
// as 1000 rather than as 0.
var _ = Describe("FirewallRule priority", func() {
	Context("Create a GCPCluster with a firewall rule", func() {
		cases := []struct {
			name          string
			priority      int
			wantErr       bool
			errorContains string
			wantPriority  int
		}{
			{
				// omitempty drops the zero value, so the field never reaches the API
				// server and the schema default fills it in.
				name:         "should default an omitted priority to 1000",
				priority:     0,
				wantPriority: 1000,
			},
			{
				name:         "should accept the minimum",
				priority:     1,
				wantPriority: 1,
			},
			{
				name:         "should preserve a priority between the bounds",
				priority:     900,
				wantPriority: 900,
			},
			{
				name:         "should accept the maximum",
				priority:     65535,
				wantPriority: 65535,
			},
			{
				name:          "should reject a priority above the maximum",
				priority:      65536,
				wantErr:       true,
				errorContains: "should be less than or equal to 65535",
			},
		}

		for i, tc := range cases {
			It(tc.name, func() {
				ctx := context.Background()

				cluster := clusterWithRule(fmt.Sprintf("test-priority-%d", i), infrav1.FirewallRule{
					Name:     fmt.Sprintf("rule-%d", i),
					Priority: tc.priority,
					Allowed: []infrav1.FirewallDescriptor{
						{IPProtocol: infrav1.FirewallProtocolTCP, Ports: []string{"22"}},
					},
				})

				err := k8sClient.Create(ctx, cluster)

				if tc.wantErr {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(tc.errorContains))
					return
				}

				Expect(err).NotTo(HaveOccurred())
				defer func() {
					Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
				}()

				stored := &infrav1.GCPCluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), stored)).To(Succeed())
				Expect(stored.Spec.Network.Firewall.FirewallRules[0].Priority).To(Equal(tc.wantPriority))
			})
		}

		// An explicit 0 is what omitempty would otherwise drop, so sending it unstructured
		// shows the minimum is enforced by the schema and not merely by the Go type. A
		// typed client cannot produce this request.
		It("should reject an explicitly set priority of 0", func() {
			ctx := context.Background()

			cluster := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": infrav1.GroupVersion.String(),
					"kind":       "GCPCluster",
					"metadata": map[string]interface{}{
						"name":      "test-priority-zero",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"project": "test-project",
						"region":  "us-central1",
						"network": map[string]interface{}{
							"firewall": map[string]interface{}{
								"firewallRules": []interface{}{
									map[string]interface{}{
										"name":     "zero-priority",
										"priority": int64(0),
										"allowed": []interface{}{
											map[string]interface{}{
												"IPProtocol": "TCP",
												"ports":      []interface{}{"22"},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			err := k8sClient.Create(ctx, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("should be greater than or equal to 1"))
		})
	})
})
