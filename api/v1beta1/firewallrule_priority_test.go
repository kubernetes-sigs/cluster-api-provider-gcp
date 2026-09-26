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

package v1beta1

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// startPriorityTestEnv brings up an API server with the generated CRDs installed, so the
// assertions below exercise the real schema rather than the Go struct tags alone.
func startPriorityTestEnv(t *testing.T, g *WithT) client.Client {
	t.Helper()

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())

	t.Cleanup(func() {
		g.Expect(testEnv.Stop()).To(Succeed())
	})

	g.Expect(AddToScheme(scheme.Scheme)).To(Succeed())

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(k8sClient).NotTo(BeNil())

	return k8sClient
}

func clusterWithRule(name string, rule FirewallRule) *GCPCluster {
	return &GCPCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: GCPClusterSpec{
			Project: "test-project",
			Region:  "us-central1",
			Network: NetworkSpec{
				Firewall: FirewallSpec{
					FirewallRules: []FirewallRule{rule},
				},
			},
		},
	}
}

// TestFirewallRule_Priority covers the interaction between the omitempty json tag, the
// minimum of 1 and the default of 1000. The only value omitempty drops is 0, and 0 is
// the one value the minimum rejects, so nothing a user can express is lost: an omitted
// priority is stored as 1000 rather than as 0.
func TestFirewallRule_Priority(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	k8sClient := startPriorityTestEnv(t, g)

	tests := []struct {
		name          string
		priority      int32
		wantErr       bool
		errorContains string
		wantPriority  int32
	}{
		{
			// omitempty drops the zero value, so the field never reaches the API
			// server and the schema default fills it in.
			name:         "an omitted priority is defaulted to 1000",
			priority:     0,
			wantPriority: 1000,
		},
		{
			name:         "the minimum is accepted",
			priority:     1,
			wantPriority: 1,
		},
		{
			name:         "a priority between the bounds is preserved",
			priority:     900,
			wantPriority: 900,
		},
		{
			name:         "the maximum is accepted",
			priority:     65535,
			wantPriority: 65535,
		},
		{
			name:          "a priority above the maximum is rejected",
			priority:      65536,
			wantErr:       true,
			errorContains: "should be less than or equal to 65535",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cluster := clusterWithRule(fmt.Sprintf("test-priority-%d", i), FirewallRule{
				Name:     fmt.Sprintf("rule-%d", i),
				Priority: tt.priority,
				Allowed: []FirewallDescriptor{
					{IPProtocol: FirewallProtocolTCP, Ports: []string{"22"}},
				},
			})

			err := k8sClient.Create(ctx, cluster)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.errorContains))
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			t.Cleanup(func() { _ = k8sClient.Delete(ctx, cluster) })

			stored := &GCPCluster{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), stored)).To(Succeed())
			g.Expect(stored.Spec.Network.Firewall.FirewallRules[0].Priority).To(Equal(tt.wantPriority))
		})
	}
}

// TestFirewallRule_PriorityZeroIsRejected sends an explicit 0 that omitempty would
// otherwise drop, to show the minimum is enforced by the schema and not merely by the
// Go type. A typed client cannot produce this request.
func TestFirewallRule_PriorityZeroIsRejected(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	k8sClient := startPriorityTestEnv(t, g)

	cluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": GroupVersion.String(),
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
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("should be greater than or equal to 1"))
}
