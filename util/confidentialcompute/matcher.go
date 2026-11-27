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

// Package confidentialcompute implements checks for which machine types support Confidential VM.
package confidentialcompute

import (
	"fmt"
	"strings"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

// Confidential VM Technology support depends on the configured machine types.
// reference: https://docs.cloud.google.com/confidential-computing/confidential-vm/docs/supported-configurations#machine-type-cpu-zone.
// Last checked: 23/10/2026
var confidentialMachineSeriesPatterns = map[infrav1.ConfidentialComputePolicy][]string{
	infrav1.ConfidentialComputePolicySEV:    {"n2d-", "c2d-", "c3d-", "c4d-", "g4-standard-48"},
	infrav1.ConfidentialComputePolicySEVSNP: {"n2d-"},
	infrav1.ConfidentialComputePolicyTDX:    {"c3-", "c4-", "a3-highgpu-1g"},
}

// Matcher is a struct to allow us to quickly calculate if confidential computing is supported by
// specific instance types. Use the NewMatcher function to get it with the supported set of instances
type Matcher struct {
	patterns map[infrav1.ConfidentialComputePolicy][]string
}

// NewMatcher produces a new Matcher with the supported instance series and types
// construct the struct directly for testing purposes.
func NewMatcher() Matcher {
	return Matcher{
		patterns: confidentialMachineSeriesPatterns,
	}
}

// Match checks if the machineType given has a match in the set of instanceTypes we know support
// each infrav1.ConfidentialComputePolicy. `nil` here means that the policy is not supported, whereas false
// means the policy is supported but not on that machine.
func (c *Matcher) Match(policy *infrav1.ConfidentialComputePolicy, machineType string) *bool {
	policyForMap := c.getPolicyForMap(policy)
	if _, exists := c.patterns[policyForMap]; exists {
		for _, pattern := range c.patterns[policyForMap] {
			if strings.HasPrefix(machineType, pattern) {
				return new(true)
			}
		}
		return new(false)
	}
	return nil
}

func (c *Matcher) splitSeriesAndInstances(policy *infrav1.ConfidentialComputePolicy) ([]string, []string) {
	policyForMap := c.getPolicyForMap(policy)
	patterns := c.patterns[policyForMap]
	var series []string
	var instanceTypes []string
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "-") {
			series = append(series, pattern+"*")
		} else {
			instanceTypes = append(instanceTypes, pattern)
		}
	}
	return series, instanceTypes
}

// GetDisplayableSupportedMachines produces a message that describes the series and instances that support
// confidential computing.
func (c *Matcher) GetDisplayableSupportedMachines(policy *infrav1.ConfidentialComputePolicy, instanceType string) string {
	series, instanceTypes := c.splitSeriesAndInstances(policy)
	result := fmt.Sprintf("instanceType was '%s', but ConfidentialCompute requires either a machine in the following series: '%s'", instanceType, strings.Join(series, ","))
	if len(instanceTypes) != 0 {
		result = fmt.Sprintf("%s, or an instance of the following types: %s", result, strings.Join(instanceTypes, ","))
	}
	return result
}

func (c *Matcher) getPolicyForMap(policy *infrav1.ConfidentialComputePolicy) infrav1.ConfidentialComputePolicy {
	policyForMap := *policy
	if policyForMap == infrav1.ConfidentialComputePolicyEnabled {
		policyForMap = infrav1.ConfidentialComputePolicySEV
	}
	return policyForMap
}
