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
