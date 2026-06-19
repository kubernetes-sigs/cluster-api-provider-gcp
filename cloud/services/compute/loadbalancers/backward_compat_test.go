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

package loadbalancers

import (
	"testing"

	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

// TestBackwardCompatibility_DefaultBehavior verifies that the default behavior
// remains unchanged for backward compatibility. When no LoadBalancerType is
// specified, the system defaults to External (Global External Proxy LB).
//
// The shouldCreateExternalLoadBalancer helper here targets only the global
// external path. Regional external types route through
// isRegionalExternalLoadBalancer, so both wantGlobalExternal and wantRegionalExt
// are tracked separately.
func TestBackwardCompatibility_DefaultBehavior(t *testing.T) {
	tests := []struct {
		name               string
		lbSpec             infrav1.LoadBalancerSpec
		wantType           infrav1.LoadBalancerType
		wantGlobalExternal bool
		wantRegionalExt    bool
		wantInternal       bool
	}{
		{
			name: "nil LoadBalancerType defaults to External",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: nil,
			},
			wantType:           infrav1.External,
			wantGlobalExternal: true,
			wantRegionalExt:    false,
			wantInternal:       false,
		},
		{
			name:               "empty LoadBalancerSpec defaults to External",
			lbSpec:             infrav1.LoadBalancerSpec{},
			wantType:           infrav1.External,
			wantGlobalExternal: true,
			wantRegionalExt:    false,
			wantInternal:       false,
		},
		{
			name: "explicit External type uses global path",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.External),
			},
			wantType:           infrav1.External,
			wantGlobalExternal: true,
			wantRegionalExt:    false,
			wantInternal:       false,
		},
		{
			name: "InternalExternal creates Global External + Regional Internal",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.InternalExternal),
			},
			wantType:           infrav1.InternalExternal,
			wantGlobalExternal: true,
			wantRegionalExt:    false,
			wantInternal:       true,
		},
		{
			name: "Internal creates only Regional Internal Passthrough LB",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.Internal),
			},
			wantType:           infrav1.Internal,
			wantGlobalExternal: false,
			wantRegionalExt:    false,
			wantInternal:       true,
		},
		{
			name: "RegionalExternal creates Regional External Proxy LB",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalExternal),
			},
			wantType:           infrav1.RegionalExternal,
			wantGlobalExternal: false,
			wantRegionalExt:    true,
			wantInternal:       false,
		},
		{
			name: "RegionalInternalExternal creates Regional External + Regional Internal",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalInternalExternal),
			},
			wantType:           infrav1.RegionalInternalExternal,
			wantGlobalExternal: false,
			wantRegionalExt:    true,
			wantInternal:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType := ptr.Deref(tt.lbSpec.LoadBalancerType, infrav1.External)
			if gotType != tt.wantType {
				t.Errorf("Default type = %v, want %v", gotType, tt.wantType)
			}

			if got := shouldCreateExternalLoadBalancer(gotType); got != tt.wantGlobalExternal {
				t.Errorf("shouldCreateExternalLoadBalancer() = %v, want %v", got, tt.wantGlobalExternal)
			}

			if got := isRegionalExternalLoadBalancer(gotType); got != tt.wantRegionalExt {
				t.Errorf("isRegionalExternalLoadBalancer() = %v, want %v", got, tt.wantRegionalExt)
			}

			if got := shouldCreateInternalLoadBalancer(gotType); got != tt.wantInternal {
				t.Errorf("shouldCreateInternalLoadBalancer() = %v, want %v", got, tt.wantInternal)
			}
		})
	}
}

// TestBackwardCompatibility_ExistingBehavior verifies that existing LB types
// continue to route to their original code paths and the new RegionalExternal
// types route to the new path.
func TestBackwardCompatibility_ExistingBehavior(t *testing.T) {
	tests := []struct {
		name         string
		lbType       infrav1.LoadBalancerType
		wantRegional bool
	}{
		{
			name:         "External routes to global external LB",
			lbType:       infrav1.External,
			wantRegional: false,
		},
		{
			name:         "InternalExternal routes to global external LB",
			lbType:       infrav1.InternalExternal,
			wantRegional: false,
		},
		{
			name:         "RegionalExternal routes to regional external LB",
			lbType:       infrav1.RegionalExternal,
			wantRegional: true,
		},
		{
			name:         "RegionalInternalExternal routes to regional external LB",
			lbType:       infrav1.RegionalInternalExternal,
			wantRegional: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRegionalExternalLoadBalancer(tt.lbType); got != tt.wantRegional {
				t.Errorf("isRegionalExternalLoadBalancer() = %v, want %v", got, tt.wantRegional)
			}
		})
	}
}

// TestBackwardCompatibility_DefaultNaming verifies that the internal LB name
// resolution preserves the original default behavior. The external name in
// this branch is always the APIServer role tag (no customization API), so it
// is not parametrized.
func TestBackwardCompatibility_DefaultNaming(t *testing.T) {
	tests := []struct {
		name     string
		lbSpec   infrav1.LoadBalancerSpec
		wantName string
	}{
		{
			name: "Internal LB defaults to api-internal name when InternalLoadBalancer is empty",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: &infrav1.LoadBalancer{},
			},
			wantName: infrav1.InternalRoleTagValue,
		},
		{
			name:     "Internal LB defaults to api-internal name when InternalLoadBalancer is nil",
			lbSpec:   infrav1.LoadBalancerSpec{},
			wantName: infrav1.InternalRoleTagValue,
		},
		{
			name: "Custom internal LB name is respected",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: &infrav1.LoadBalancer{
					Name: ptr.To("custom-internal"),
				},
			},
			wantName: "custom-internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getInternalLoadBalancerName(tt.lbSpec); got != tt.wantName {
				t.Errorf("getInternalLoadBalancerName() = %v, want %v", got, tt.wantName)
			}
		})
	}
}
