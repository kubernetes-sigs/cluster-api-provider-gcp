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
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var (
	minCount          = int32(1)
	maxCount          = int32(3)
	invalidMinCount   = int32(-1)
	enableAutoscaling = false
	diskSizeGb        = int32(200)
	maxPods           = int64(10)
	localSsds         = int32(0)
	invalidDiskSizeGb = int32(-200)
	invalidMaxPods    = int64(-10)
	invalidLocalSsds  = int32(-0)
)

func TestGCPManagedMachinePoolValidatingWebhookCreate(t *testing.T) {
	tests := []struct {
		name        string
		spec        GCPManagedMachinePoolSpec
		expectError bool
	}{
		{
			name: "valid node pool name",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
			},
			expectError: false,
		},
		{
			name: "node pool name is too long",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: strings.Repeat("A", maxNodePoolNameLength+1),
			},
			expectError: true,
		},
		{
			name: "scaling with valid min/max count",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				Scaling: &NodePoolAutoScaling{
					MinCount: &minCount,
					MaxCount: &maxCount,
				},
			},
			expectError: false,
		},
		{
			name: "scaling with invalid min/max count",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				Scaling: &NodePoolAutoScaling{
					MinCount: &invalidMinCount,
					MaxCount: &maxCount,
				},
			},
			expectError: true,
		},
		{
			name: "scaling with max < min count",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				Scaling: &NodePoolAutoScaling{
					MinCount: &maxCount,
					MaxCount: &minCount,
				},
			},
			expectError: true,
		},
		{
			name: "autoscaling disabled and min/max provided",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				Scaling: &NodePoolAutoScaling{
					EnableAutoscaling: &enableAutoscaling,
					MinCount:          &minCount,
					MaxCount:          &maxCount,
				},
			},
			expectError: true,
		},
		{
			name: "valid non-negative values",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName:   "nodepool1",
				DiskSizeGb:     &diskSizeGb,
				MaxPodsPerNode: &maxPods,
				LocalSsdCount:  &localSsds,
			},
			expectError: false,
		},
		{
			name: "invalid negative values",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName:   "nodepool1",
				DiskSizeGb:     &invalidDiskSizeGb,
				MaxPodsPerNode: &invalidMaxPods,
				LocalSsdCount:  &invalidLocalSsds,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mmp := &GCPManagedMachinePool{
				Spec: tc.spec,
			}
			warn, err := mmp.ValidateCreate()

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

func TestGCPManagedMachinePoolValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		spec        GCPManagedMachinePoolSpec
		expectError bool
	}{
		{
			name: "node pool is not mutated",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
			},
			expectError: false,
		},
		{
			name: "mutable fields are mutated",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				AdditionalLabels: infrav1.Labels{
					"testKey": "testVal",
				},
			},
			expectError: false,
		},
		{
			name: "immutable field disk size is mutated",
			spec: GCPManagedMachinePoolSpec{
				NodePoolName: "nodepool1",
				DiskSizeGb:   &diskSizeGb,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMMP := &GCPManagedMachinePool{
				Spec: tc.spec,
			}
			oldMMP := &GCPManagedMachinePool{
				Spec: GCPManagedMachinePoolSpec{
					NodePoolName: "nodepool1",
				},
			}

			warn, err := newMMP.ValidateUpdate(oldMMP)

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
