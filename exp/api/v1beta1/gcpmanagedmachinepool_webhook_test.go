/*
Copyright 2023 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var gcpmmp *GCPManagedMachinePool

var _ = Describe("Test GCPManagedMachinePool Webhooks", func() {
	BeforeEach(func() {
		machineType := "e2-medium"
		diskSizeGb := int32(100)

		gcpmmp = &GCPManagedMachinePool{
			Spec: GCPManagedMachinePoolSpec{
				NodePoolName: "test-gke-pool",
				MachineType:  &machineType,
				DiskSizeGb:   &diskSizeGb,
			},
		}
	})

	Context("Test validateSpec", func() {
		It("should error when node pool name is too long", func() {
			gcpmmp.Spec.NodePoolName = strings.Repeat("A", maxNodePoolNameLength+1)
			errs := gcpmmp.validateSpec()
			Expect(errs).ToNot(BeEmpty())
		})
		It("should pass when node pool name is within limit", func() {
			gcpmmp.Spec.NodePoolName = strings.Repeat("A", maxNodePoolNameLength)
			errs := gcpmmp.validateSpec()
			Expect(errs).To(BeEmpty())
		})
	})

	Context("Test validateScaling", func() {
		It("should pass when scaling is not specified", func() {
			errs := gcpmmp.validateScaling()
			Expect(errs).To(BeEmpty())
		})
		It("should pass when min/max count is valid", func() {
			minCount := int32(1)
			maxCount := int32(3)
			gcpmmp.Spec.Scaling = &NodePoolAutoScaling{
				MinCount: &minCount,
				MaxCount: &maxCount,
			}

			errs := gcpmmp.validateScaling()
			Expect(errs).To(BeEmpty())
		})
		It("should fail when min is negative", func() {
			minCount := int32(-1)
			gcpmmp.Spec.Scaling = &NodePoolAutoScaling{
				MinCount: &minCount,
			}

			errs := gcpmmp.validateScaling()
			Expect(errs).ToNot(BeEmpty())
		})
		It("should fail when min > max", func() {
			minCount := int32(3)
			maxCount := int32(1)
			gcpmmp.Spec.Scaling = &NodePoolAutoScaling{
				MinCount: &minCount,
				MaxCount: &maxCount,
			}

			errs := gcpmmp.validateScaling()
			Expect(errs).ToNot(BeEmpty())
		})
		It("should fail when autoscaling is disabled and min/max is specified", func() {
			minCount := int32(1)
			maxCount := int32(3)
			enabled := false
			locationPolicy := ManagedNodePoolLocationPolicyAny
			gcpmmp.Spec.Scaling = &NodePoolAutoScaling{
				MinCount:          &minCount,
				MaxCount:          &maxCount,
				EnableAutoscaling: &enabled,
				LocationPolicy:    &locationPolicy,
			}

			errs := gcpmmp.validateScaling()
			Expect(errs).To(HaveLen(3))
		})
	})
	Context("Test validateImmutable", func() {
		It("should pass when node pool is not mutated", func() {
			old := gcpmmp.DeepCopy()
			errs := gcpmmp.validateImmutable(old)
			Expect(errs).To(BeEmpty())
		})
		It("should pass when mutable fields are mutated", func() {
			old := gcpmmp.DeepCopy()
			gcpmmp.Spec.AdditionalLabels = infrav1.Labels{
				"testKey": "testVal",
			}

			errs := gcpmmp.validateImmutable(old)
			Expect(errs).To(BeEmpty())
		})
		It("should fail when immutable fields are mutated", func() {
			old := gcpmmp.DeepCopy()
			diskSizeGb := int32(200)
			gcpmmp.Spec.DiskSizeGb = &diskSizeGb
			gcpmmp.Spec.NodePoolName = "new-name"
			gcpmmp.Spec.Management = &NodePoolManagement{
				AutoUpgrade: false,
				AutoRepair:  false,
			}

			errs := gcpmmp.validateImmutable(old)
			Expect(errs).To(HaveLen(3))
		})
	})

	Context("Test validateNonNegative", func() {
		It("should pass when number fields are not specified", func() {
			errs := gcpmmp.validateNonNegative()
			Expect(errs).To(BeEmpty())
		})
		It("should pass when number fields are non-negative", func() {
			maxPods := int64(10)
			localSsds := int32(0)
			diskSize := int32(200)
			gcpmmp.Spec.MaxPodsPerNode = &maxPods
			gcpmmp.Spec.LocalSsdCount = &localSsds
			gcpmmp.Spec.DiskSizeGb = &diskSize

			errs := gcpmmp.validateNonNegative()
			Expect(errs).To(BeEmpty())
		})
		It("should pass when some number fields are negative", func() {
			maxPods := int64(-1)
			localSsds := int32(0)
			diskSize := int32(-100)
			gcpmmp.Spec.MaxPodsPerNode = &maxPods
			gcpmmp.Spec.LocalSsdCount = &localSsds
			gcpmmp.Spec.DiskSizeGb = &diskSize

			errs := gcpmmp.validateNonNegative()
			Expect(errs).To(HaveLen(2))
		})
	})
})
