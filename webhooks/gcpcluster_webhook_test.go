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
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestGCPCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *infrav1.GCPCluster
		wantErr     bool
		wantWarning bool
	}{
		{
			name:    "empty spec passes",
			cluster: &infrav1.GCPCluster{},
		},
		{
			name: "unknown load balancer type is rejected",
			cluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.LoadBalancerType("Weird")),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ExternalLoadBalancerConfig with Internal LB emits warning",
			cluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.Internal),
						ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
							Name: ptr.To("ignored"),
						},
					},
				},
			},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			warn, err := (&GCPCluster{}).ValidateCreate(t.Context(), tt.cluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if tt.wantWarning {
				g.Expect(warn).NotTo(BeEmpty())
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}

func TestGCPCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		newCluster  *infrav1.GCPCluster
		oldCluster  *infrav1.GCPCluster
		wantErr     bool
		wantWarning bool
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
		{
			name: "invalid LoadBalancerType is rejected on update",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{Mtu: int64(1500)},
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.LoadBalancerType("Weird")),
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{Mtu: int64(1500)},
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.LoadBalancerType("Weird")),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ExternalLoadBalancerConfig with Internal LB emits warning on update",
			newCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{Mtu: int64(1500)},
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.Internal),
						ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
							Name: ptr.To("ignored"),
						},
					},
				},
			},
			oldCluster: &infrav1.GCPCluster{
				Spec: infrav1.GCPClusterSpec{
					Network: infrav1.NetworkSpec{Mtu: int64(1500)},
					LoadBalancer: infrav1.LoadBalancerSpec{
						LoadBalancerType: ptr.To(infrav1.Internal),
						ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
							Name: ptr.To("ignored"),
						},
					},
				},
			},
			wantWarning: true,
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
			if test.wantWarning {
				g.Expect(warn).NotTo(BeEmpty())
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}
