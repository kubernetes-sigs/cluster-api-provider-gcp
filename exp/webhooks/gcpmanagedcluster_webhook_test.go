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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func TestGCPManagedClusterValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		spec        expinfrav1.GCPManagedClusterSpec
	}{
		{
			name:        "request to change mutable field additional labels",
			expectError: false,
			spec: expinfrav1.GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-west1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
				AdditionalLabels: map[string]string{
					"testKey": "testVal",
				},
			},
		},
		{
			name:        "request to change immutable field project",
			expectError: true,
			spec: expinfrav1.GCPManagedClusterSpec{
				Project: "new-project",
				Region:  "us-west1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
			},
		},
		{
			name:        "request to change immutable field region",
			expectError: true,
			spec: expinfrav1.GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-central1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
			},
		},
		{
			name:        "request to change immutable field credentials ref",
			expectError: true,
			spec: expinfrav1.GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-central1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "new-ns",
					Name:      "new-name",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMC := &expinfrav1.GCPManagedCluster{
				Spec: tc.spec,
			}
			oldMC := &expinfrav1.GCPManagedCluster{
				Spec: expinfrav1.GCPManagedClusterSpec{
					Project: "old-project",
					Region:  "us-west1",
					CredentialsRef: &infrav1.ObjectReference{
						Namespace: "default",
						Name:      "credsref",
					},
				},
			}

			warn, err := (&GCPManagedCluster{}).ValidateUpdate(t.Context(), oldMC, newMC)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			// Nothing emits warnings yet
			g.Expect(warn).To(BeEmpty())
		})
	}
}

// TestGCPManagedClusterValidatingWebhookUpdateRatchetsDuplicateFirewallRules covers the
// clusters that stored indistinguishable rules before the duplicate check existed.
// Rejecting those on every update would strand them, because an unrelated change would
// be refused over rules the user never touched.
func TestGCPManagedClusterValidatingWebhookUpdateRatchetsDuplicateFirewallRules(t *testing.T) {
	duplicated := infrav1.FirewallRule{
		Name:      "ssh",
		Direction: infrav1.FirewallRuleDirectionIngress,
		Priority:  1000,
		Allowed: []infrav1.FirewallDescriptor{
			{IPProtocol: infrav1.FirewallProtocolTCP, Ports: []string{"22"}},
		},
	}
	valid := infrav1.FirewallRule{Name: "https", Direction: infrav1.FirewallRuleDirectionIngress}

	tests := []struct {
		name             string
		oldRules         []infrav1.FirewallRule
		newRules         []infrav1.FirewallRule
		additionalLabels map[string]string
		expectError      bool
	}{
		{
			name:             "duplicates that are left alone do not block an unrelated change",
			oldRules:         []infrav1.FirewallRule{duplicated, duplicated},
			newRules:         []infrav1.FirewallRule{duplicated, duplicated},
			additionalLabels: map[string]string{"testKey": "testVal"},
			expectError:      false,
		},
		{
			name:        "duplicates that survive an edit to the rules are rejected",
			oldRules:    []infrav1.FirewallRule{duplicated, duplicated},
			newRules:    []infrav1.FirewallRule{duplicated, duplicated, valid},
			expectError: true,
		},
		{
			name:        "an edit that resolves the duplicates is accepted",
			oldRules:    []infrav1.FirewallRule{duplicated, duplicated},
			newRules:    []infrav1.FirewallRule{duplicated, valid},
			expectError: false,
		},
		{
			name:        "duplicates introduced by the update are rejected",
			oldRules:    []infrav1.FirewallRule{valid},
			newRules:    []infrav1.FirewallRule{duplicated, duplicated},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			managedCluster := func(labels map[string]string, rules []infrav1.FirewallRule) *expinfrav1.GCPManagedCluster {
				return &expinfrav1.GCPManagedCluster{
					Spec: expinfrav1.GCPManagedClusterSpec{
						Project: "old-project",
						Region:  "us-west1",
						CredentialsRef: &infrav1.ObjectReference{
							Namespace: "default",
							Name:      "credsref",
						},
						AdditionalLabels: labels,
						Network: infrav1.NetworkSpec{
							Firewall: infrav1.FirewallSpec{FirewallRules: rules},
						},
					},
				}
			}

			_, err := (&GCPManagedCluster{}).ValidateUpdate(t.Context(),
				managedCluster(nil, tc.oldRules), managedCluster(tc.additionalLabels, tc.newRules))

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
