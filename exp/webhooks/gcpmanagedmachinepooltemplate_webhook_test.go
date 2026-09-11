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

func TestGCPManagedMachinePoolTemplateValidatingWebhookCreateInstanceTypeDeprecationWarning(t *testing.T) {
	g := NewWithT(t)

	instanceType := "n1-standard-1"
	mmpt := &expinfrav1.GCPManagedMachinePoolTemplate{
		Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
			Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
				Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
					GCPManagedMachinePoolClassSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
						NodePoolName: "nodepool1",
						InstanceType: &instanceType,
					},
				},
			},
		},
	}

	warn, err := (&GCPManagedMachinePoolTemplate{}).ValidateCreate(t.Context(), mmpt)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(warn).To(ConsistOf(ContainSubstring("spec.template.spec.instanceType is deprecated")))
}
