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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func TestGCPManagedMachinePoolTemplateValidatingWebhookCreate(t *testing.T) {
	tests := []struct {
		name       string
		classSpec  expinfrav1.GCPManagedMachinePoolClassSpec
		expectWarn bool
	}{
		{
			name: "valid node pool template",
			classSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
				NodePoolName: "nodepool1",
			},
			expectWarn: false,
		},
		{
			name: "diskSizeGB (deprecated) set should cause a warning",
			classSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
				NodePoolName: "nodepool1",
				DiskSizeGB:   &diskSizeGB,
			},
			expectWarn: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mmpt := &expinfrav1.GCPManagedMachinePoolTemplate{
				Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
					Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
						Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
							GCPManagedMachinePoolClassSpec: tc.classSpec,
						},
					},
				},
			}

			warn, err := (&GCPManagedMachinePoolTemplate{}).ValidateCreate(t.Context(), mmpt)

			g.Expect(err).ToNot(HaveOccurred())
			if tc.expectWarn {
				g.Expect(warn).ToNot(BeEmpty())
			} else {
				g.Expect(warn).To(BeEmpty())
			}
		})
	}
}

func TestGCPManagedMachinePoolTemplateValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		classSpec   expinfrav1.GCPManagedMachinePoolClassSpec
		expectError bool
	}{
		{
			name: "node pool template is not mutated",
			classSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
				NodePoolName: "nodepool1",
			},
			expectError: false,
		},
		{
			name: "immutable field diskSizeGB (deprecated) is mutated",
			classSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
				NodePoolName: "nodepool1",
				DiskSizeGB:   &diskSizeGB,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMMPT := &expinfrav1.GCPManagedMachinePoolTemplate{
				Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
					Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
						Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
							GCPManagedMachinePoolClassSpec: tc.classSpec,
						},
					},
				},
			}
			oldMMPT := &expinfrav1.GCPManagedMachinePoolTemplate{
				Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
					Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
						Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
							GCPManagedMachinePoolClassSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
								NodePoolName: "nodepool1",
							},
						},
					},
				},
			}

			warn, err := (&GCPManagedMachinePoolTemplate{}).ValidateUpdate(t.Context(), oldMMPT, newMMPT)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			g.Expect(warn).To(BeEmpty())
		})
	}
}
