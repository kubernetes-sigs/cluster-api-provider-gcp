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

package v1beta1

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	vV1_27_1       = "v1.27.1"
	releaseChannel = Rapid
)

func TestGCPManagedControlPlaneDefaultingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		resourceNS   string
		spec         GCPManagedControlPlaneSpec
		expectSpec   GCPManagedControlPlaneSpec
		expetError   bool
		expectHash   bool
	}{
		{
			name:         "valid cluster name",
			resourceName: "cluster1",
			resourceNS:   "default",
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "default_cluster1",
			},
			expectSpec: GCPManagedControlPlaneSpec{ClusterName: "default_cluster1"},
		},
		{
			name:         "no cluster name should generate a valid one",
			resourceName: "cluster1",
			resourceNS:   "default",
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "",
			},
			expectSpec: GCPManagedControlPlaneSpec{ClusterName: "default-cluster1"},
		},
		{
			name:         "invalid cluster name (too long)",
			resourceName: strings.Repeat("A", maxClusterNameLength+1),
			resourceNS:   "default",
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "",
			},
			expectSpec: GCPManagedControlPlaneSpec{ClusterName: "capg-"},
			expectHash: true,
		},
		{
			name:         "with kubernetes version",
			resourceName: "cluster1",
			resourceNS:   "default",
			spec: GCPManagedControlPlaneSpec{
				ClusterName:         "cluster1_27_1",
				ControlPlaneVersion: &vV1_27_1,
			},
			expectSpec: GCPManagedControlPlaneSpec{ClusterName: "cluster1_27_1", ControlPlaneVersion: &vV1_27_1},
		},
		{
			name:         "with autopilot enabled",
			resourceName: "cluster1",
			resourceNS:   "default",
			spec: GCPManagedControlPlaneSpec{
				ClusterName:         "cluster1_autopilot",
				ControlPlaneVersion: &vV1_27_1,
				EnableAutopilot:     true,
			},
			expectSpec: GCPManagedControlPlaneSpec{ClusterName: "cluster1_autopilot", ControlPlaneVersion: &vV1_27_1, EnableAutopilot: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mcp := &GCPManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tc.resourceName,
					Namespace: tc.resourceNS,
				},
				Spec: tc.spec,
			}
			mcp.Default()

			g.Expect(mcp.Spec).ToNot(BeNil())
			g.Expect(mcp.Spec.ClusterName).ToNot(BeEmpty())

			if tc.expectHash {
				g.Expect(strings.HasPrefix(mcp.Spec.ClusterName, "capg-")).To(BeTrue())
				// We don't care about the exact name
				tc.expectSpec.ClusterName = mcp.Spec.ClusterName
			}
			g.Expect(mcp.Spec).To(Equal(tc.expectSpec))
		})
	}
}

func TestGCPManagedControlPlaneValidatingWebhookCreate(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		spec        GCPManagedControlPlaneSpec
	}{
		{
			name:        "cluster name too long should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName: strings.Repeat("A", maxClusterNameLength+1),
			},
		},
		{
			name:        "autopilot enabled without release channel should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName:     "",
				EnableAutopilot: true,
				ReleaseChannel:  nil,
			},
		},
		{
			name:        "autopilot enabled with release channel",
			expectError: false,
			spec: GCPManagedControlPlaneSpec{
				ClusterName:     "",
				EnableAutopilot: true,
				ReleaseChannel:  &releaseChannel,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mcp := &GCPManagedControlPlane{
				Spec: tc.spec,
			}
			warn, err := mcp.ValidateCreate()

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

func TestGCPManagedControlPlaneValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		spec        GCPManagedControlPlaneSpec
	}{
		{
			name:        "request to change cluster name should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "default_cluster2",
			},
		},
		{
			name:        "request to change project should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "default_cluster1",
				Project:     "new-project",
			},
		},
		{
			name:        "request to change location should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "default_cluster1",
				Location:    "us-west4",
			},
		},
		{
			name:        "request to enable/disable autopilot should cause an error",
			expectError: true,
			spec: GCPManagedControlPlaneSpec{
				ClusterName:     "default_cluster1",
				EnableAutopilot: true,
			},
		},
		{
			name:        "request to change network should not cause an error",
			expectError: false,
			spec: GCPManagedControlPlaneSpec{
				ClusterName: "default_cluster1",
				ClusterNetwork: &ClusterNetwork{
					PrivateCluster: &PrivateCluster{
						EnablePrivateEndpoint: false,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMCP := &GCPManagedControlPlane{
				Spec: tc.spec,
			}
			oldMCP := &GCPManagedControlPlane{
				Spec: GCPManagedControlPlaneSpec{
					ClusterName: "default_cluster1",
					ClusterNetwork: &ClusterNetwork{
						PrivateCluster: &PrivateCluster{
							EnablePrivateEndpoint: true,
						},
					},
				},
			}

			warn, err := newMCP.ValidateUpdate(oldMCP)

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
