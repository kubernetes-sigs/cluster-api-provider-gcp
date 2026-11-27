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

package confidentialcompute

import (
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestMatcher_Match(t *testing.T) {
	tests := []struct {
		name        string
		policy      infrav1.ConfidentialComputePolicy
		machineType string
		want        *bool
	}{
		{
			name:        "test prefix boundaries positive TDX",
			policy:      infrav1.ConfidentialComputePolicyTDX,
			machineType: "c3-standard-4",
			want:        new(true),
		},
		{
			name:        "test prefix boundaries negative TDX",
			policy:      infrav1.ConfidentialComputePolicyTDX,
			machineType: "c3d-standard-4",
			want:        new(false),
		},
		{
			name:        "test prefix boundaries negative SEV",
			policy:      infrav1.ConfidentialComputePolicySEV,
			machineType: "c4-standard-8",
			want:        new(false),
		},
		{
			name:        "test prefix boundaries positive SEV",
			policy:      infrav1.ConfidentialComputePolicySEV,
			machineType: "c4d-standard-8",
			want:        new(true),
		},
		{
			name:        "exact name entries accept",
			policy:      infrav1.ConfidentialComputePolicyTDX,
			machineType: "a3-highgpu-1g",
			want:        new(true),
		},
		{
			name:        "exact name entries reject",
			policy:      infrav1.ConfidentialComputePolicyTDX,
			machineType: "a3-highgpu-8g",
			want:        new(false),
		},
		{
			name:        "unrecognised policy returns nil",
			policy:      "foobar",
			machineType: "a3-highgpu-8g",
			want:        nil,
		},
		{
			name:        "disabled returns nil",
			policy:      infrav1.ConfidentialComputePolicyDisabled,
			machineType: "a3-highgpu-8g",
			want:        nil,
		},
		{
			name:        "empty name is false",
			policy:      infrav1.ConfidentialComputePolicyTDX,
			machineType: "",
			want:        new(false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewMatcher()
			if got := c.Match(new(tt.policy), tt.machineType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatcher_GetDisplayableSupportedMachines(t *testing.T) {
	g := gomega.NewWithT(t)
	c := &Matcher{
		patterns: map[infrav1.ConfidentialComputePolicy][]string{
			infrav1.ConfidentialComputePolicyTDX: {"a2d-", "a2d-standard-4", "a4d-", "c3p-large-9"},
		},
	}
	series, instances := c.splitSeriesAndInstances(new(infrav1.ConfidentialComputePolicyTDX))
	g.Expect(series).To(gomega.ConsistOf([]string{"a2d-*", "a4d-*"}))
	g.Expect(instances).To(gomega.ConsistOf([]string{"a2d-standard-4", "c3p-large-9"}))
}
