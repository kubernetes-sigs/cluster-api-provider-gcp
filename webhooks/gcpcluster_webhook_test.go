/*
Copyright 2021 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	firewallutil "sigs.k8s.io/cluster-api-provider-gcp/util/firewall"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestGCPCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name       string
		newCluster *infrav1.GCPCluster
		oldCluster *infrav1.GCPCluster
		wantErr    bool
	}{
		{
			name: "GCPCluster with MTU field is within the limits of more than 1300 and less than 8896",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1400),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPCluster with MTU field more than 8896",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(10000),
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPCluster with MTU field less than 8896",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1250),
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPCluster with Firewall with Allowed field wrong protocol for ports",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Allowed: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolESP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Allowed: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolESP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPCluster with Firewall with Denied field wrong protocol for ports",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Denied: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolESP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Denied: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolESP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPCluster with Firewall with Allowed field correct protocol for ports",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Allowed: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolTCP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Mtu: int64(1500),
						Firewall: infrav1.FirewallSpec{
							FirewallRules: []infrav1.FirewallRule{
								{
									Allowed: []infrav1.FirewallDescriptor{
										{
											IPProtocol: infrav1.FirewallProtocolTCP,
											Ports:      []string{"1234"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			warn, err := (&GCPCluster{}).ValidateUpdate(t.Context(), test.oldCluster, test.newCluster)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}

func TestGCPCluster_Default(t *testing.T) {
	g := NewWithT(t)

	cluster := &infrav1.GCPCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
		Spec: infrav1.GCPClusterSpec{
			Network: infrav1.NetworkSpec{
				Firewall: infrav1.FirewallSpec{
					FirewallRules: []infrav1.FirewallRule{
						{
							Name:      "explicit-name",
							Direction: infrav1.FirewallRuleDirectionIngress,
						},
						{
							Direction: infrav1.FirewallRuleDirectionIngress,
							Allowed: []infrav1.FirewallDescriptor{
								{IPProtocol: infrav1.FirewallProtocolTCP, Ports: []string{"443"}},
							},
						},
					},
				},
			},
		},
	}

	g.Expect((&GCPCluster{}).Default(t.Context(), cluster)).To(Succeed())

	rules := cluster.Spec.Network.Firewall.FirewallRules
	g.Expect(rules[0].Name).To(Equal("explicit-name"))
	g.Expect(rules[1].Name).To(HavePrefix("my-cluster-"))
	g.Expect(len(rules[1].Name)).To(BeNumerically("<=", firewallutil.MaxRuleNameLength))

	// Defaulting runs on every update, so an already defaulted rule has to keep the
	// name it was given the first time.
	generated := rules[1].Name
	g.Expect((&GCPCluster{}).Default(t.Context(), cluster)).To(Succeed())
	g.Expect(cluster.Spec.Network.Firewall.FirewallRules[1].Name).To(Equal(generated))
}

func TestGCPCluster_DefaultSkipsTopologyOwnedClusters(t *testing.T) {
	g := NewWithT(t)

	cluster := &infrav1.GCPCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "my-cluster",
			Labels: map[string]string{clusterv1.ClusterTopologyOwnedLabel: ""},
		},
		Spec: infrav1.GCPClusterSpec{
			Network: infrav1.NetworkSpec{
				Firewall: infrav1.FirewallSpec{
					FirewallRules: []infrav1.FirewallRule{
						{Direction: infrav1.FirewallRuleDirectionIngress},
					},
				},
			},
		},
	}

	g.Expect((&GCPCluster{}).Default(t.Context(), cluster)).To(Succeed())
	g.Expect(cluster.Spec.Network.Firewall.FirewallRules[0].Name).To(BeEmpty())
}

func TestGCPCluster_ValidateCreateRejectsDuplicateFirewallRules(t *testing.T) {
	duplicated := infrav1.FirewallRule{
		Direction: infrav1.FirewallRuleDirectionIngress,
		Priority:  1000,
		Allowed: []infrav1.FirewallDescriptor{
			{IPProtocol: infrav1.FirewallProtocolTCP, Ports: []string{"443"}},
		},
	}

	tests := []struct {
		name    string
		rules   []infrav1.FirewallRule
		wantErr bool
	}{
		{
			name:    "distinct rules are accepted",
			rules:   []infrav1.FirewallRule{duplicated, {Name: "ssh", Direction: infrav1.FirewallRuleDirectionIngress}},
			wantErr: false,
		},
		{
			name:    "two rules sharing a name are rejected",
			rules:   []infrav1.FirewallRule{{Name: "ssh"}, {Name: "ssh"}},
			wantErr: true,
		},
		{
			name:    "two identical unnamed rules are rejected",
			rules:   []infrav1.FirewallRule{duplicated, duplicated},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cluster := &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{
						Firewall: infrav1.FirewallSpec{FirewallRules: tt.rules},
					},
				},
			}

			_, err := (&GCPCluster{}).ValidateCreate(t.Context(), cluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

// TestGCPCluster_ValidateUpdateRejectsImmutableFirewallRuleChanges covers the fields GCP
// refuses to update on an existing rule. Admitting such a change would leave the
// reconciler retrying an update that can never succeed.
func TestGCPCluster_ValidateUpdateRejectsImmutableFirewallRuleChanges(t *testing.T) {
	ssh := infrav1.FirewallRule{
		Name:      "ssh",
		Direction: infrav1.FirewallRuleDirectionIngress,
		Priority:  1000,
		Allowed: []infrav1.FirewallDescriptor{
			{IPProtocol: infrav1.FirewallProtocolTCP, Ports: []string{"22"}},
		},
	}

	egress := ssh
	egress.Direction = infrav1.FirewallRuleDirectionEgress

	denied := ssh
	denied.Allowed, denied.Denied = nil, ssh.Allowed

	reprioritised := ssh
	reprioritised.Priority = 900

	renamed := egress
	renamed.Name = "ssh-egress"

	tests := []struct {
		name     string
		oldRules []infrav1.FirewallRule
		newRules []infrav1.FirewallRule
		wantErr  bool
	}{
		{
			name:     "a rule that changes direction under the same name is rejected",
			oldRules: []infrav1.FirewallRule{ssh},
			newRules: []infrav1.FirewallRule{egress},
			wantErr:  true,
		},
		{
			name:     "a rule that switches from allow to deny under the same name is rejected",
			oldRules: []infrav1.FirewallRule{ssh},
			newRules: []infrav1.FirewallRule{denied},
			wantErr:  true,
		},
		{
			name:     "a rule that changes direction under a new name is accepted",
			oldRules: []infrav1.FirewallRule{ssh},
			newRules: []infrav1.FirewallRule{renamed},
			wantErr:  false,
		},
		{
			name:     "a mutable change to a rule is accepted",
			oldRules: []infrav1.FirewallRule{ssh},
			newRules: []infrav1.FirewallRule{reprioritised},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cluster := func(rules []infrav1.FirewallRule) *infrav1.GCPCluster {
				return &infrav1.GCPCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
					Spec: infrav1.GCPClusterSpec{
						Network: infrav1.NetworkSpec{
							Mtu:      int64(1500),
							Firewall: infrav1.FirewallSpec{FirewallRules: rules},
						},
					},
				}
			}

			_, err := (&GCPCluster{}).ValidateUpdate(t.Context(), cluster(tt.oldRules), cluster(tt.newRules))
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

// TestGCPCluster_ValidateUpdateRatchetsDuplicateFirewallRules covers the clusters that
// stored indistinguishable rules before the duplicate check existed. Rejecting those on
// every update would strand them, because an unrelated change would be refused over
// rules the user never touched.
func TestGCPCluster_ValidateUpdateRatchetsDuplicateFirewallRules(t *testing.T) {
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
		name     string
		oldRules []infrav1.FirewallRule
		newRules []infrav1.FirewallRule
		newMtu   int64
		wantErr  bool
	}{
		{
			name:     "duplicates that are left alone do not block an unrelated change",
			oldRules: []infrav1.FirewallRule{duplicated, duplicated},
			newRules: []infrav1.FirewallRule{duplicated, duplicated},
			newMtu:   int64(1600),
			wantErr:  false,
		},
		{
			name:     "duplicates that survive an edit to the rules are rejected",
			oldRules: []infrav1.FirewallRule{duplicated, duplicated},
			newRules: []infrav1.FirewallRule{duplicated, duplicated, valid},
			wantErr:  true,
		},
		{
			name:     "an edit that resolves the duplicates is accepted",
			oldRules: []infrav1.FirewallRule{duplicated, duplicated},
			newRules: []infrav1.FirewallRule{duplicated, valid},
			wantErr:  false,
		},
		{
			name:     "duplicates introduced by the update are rejected",
			oldRules: []infrav1.FirewallRule{valid},
			newRules: []infrav1.FirewallRule{duplicated, duplicated},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			newMtu := tt.newMtu
			if newMtu == 0 {
				newMtu = int64(1500)
			}

			cluster := func(mtu int64, rules []infrav1.FirewallRule) *infrav1.GCPCluster {
				return &infrav1.GCPCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "my-cluster"},
					Spec: infrav1.GCPClusterSpec{
						Network: infrav1.NetworkSpec{
							Mtu:      mtu,
							Firewall: infrav1.FirewallSpec{FirewallRules: rules},
						},
					},
				}
			}

			_, err := (&GCPCluster{}).ValidateUpdate(t.Context(),
				cluster(int64(1500), tt.oldRules), cluster(newMtu, tt.newRules))
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}
