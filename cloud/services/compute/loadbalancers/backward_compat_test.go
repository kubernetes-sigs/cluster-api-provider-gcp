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
// remains unchanged for backward compatibility. When no LoadBalancerType is specified,
// the system should default to External (Global External Proxy Load Balancer).
func TestBackwardCompatibility_DefaultBehavior(t *testing.T) {
	tests := []struct {
		name            string
		lbSpec          infrav1.LoadBalancerSpec
		wantType        infrav1.LoadBalancerType
		wantExternal    bool
		wantRegionalExt bool
		wantInternal    bool
		description     string
	}{
		{
			name: "nil LoadBalancerType defaults to External",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: nil,
			},
			wantType:        infrav1.External,
			wantExternal:    true,
			wantRegionalExt: false,
			wantInternal:    false,
			description:     "Default behavior: creates Global External Proxy LB (original behavior)",
		},
		{
			name:            "empty LoadBalancerSpec defaults to External",
			lbSpec:          infrav1.LoadBalancerSpec{},
			wantType:        infrav1.External,
			wantExternal:    true,
			wantRegionalExt: false,
			wantInternal:    false,
			description:     "Empty spec: creates Global External Proxy LB (original behavior)",
		},
		{
			name: "explicit External type uses global path",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.External),
			},
			wantType:        infrav1.External,
			wantExternal:    true,
			wantRegionalExt: false,
			wantInternal:    false,
			description:     "Explicit External: creates Global External Proxy LB (original behavior)",
		},
		{
			name: "InternalExternal type uses original paths",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.InternalExternal),
			},
			wantType:        infrav1.InternalExternal,
			wantExternal:    true,
			wantRegionalExt: false,
			wantInternal:    true,
			description:     "InternalExternal: creates Global External + Regional Internal (original behavior)",
		},
		{
			name: "Internal type uses original path",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.Internal),
			},
			wantType:        infrav1.Internal,
			wantExternal:    false,
			wantRegionalExt: false,
			wantInternal:    true,
			description:     "Internal: creates Regional Internal Passthrough LB (original behavior)",
		},
		{
			name: "RegionalExternal type uses NEW regional path",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalExternal),
			},
			wantType:        infrav1.RegionalExternal,
			wantExternal:    false,
			wantRegionalExt: true,
			wantInternal:    false,
			description:     "NEW: RegionalExternal creates Regional External Proxy LB for GCD",
		},
		{
			name: "RegionalInternalExternal type uses NEW regional paths",
			lbSpec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalInternalExternal),
			},
			wantType:        infrav1.RegionalInternalExternal,
			wantExternal:    false,
			wantRegionalExt: true,
			wantInternal:    true,
			description:     "NEW: RegionalInternalExternal creates Regional External + Regional Internal for GCD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify default type resolution
			gotType := ptr.Deref(tt.lbSpec.LoadBalancerType, infrav1.External)
			if gotType != tt.wantType {
				t.Errorf("Default type = %v, want %v", gotType, tt.wantType)
			}

			// Verify external LB creation logic
			gotExternal := shouldCreateExternalLoadBalancer(gotType)
			if gotExternal != tt.wantExternal {
				t.Errorf("shouldCreateExternalLoadBalancer() = %v, want %v", gotExternal, tt.wantExternal)
			}

			// Verify regional external LB routing
			gotRegionalExt := isRegionalExternalLoadBalancer(gotType)
			if gotRegionalExt != tt.wantRegionalExt {
				t.Errorf("isRegionalExternalLoadBalancer() = %v, want %v", gotRegionalExt, tt.wantRegionalExt)
			}

			// Verify internal LB creation logic
			gotInternal := shouldCreateInternalLoadBalancer(gotType)
			if gotInternal != tt.wantInternal {
				t.Errorf("shouldCreateInternalLoadBalancer() = %v, want %v", gotInternal, tt.wantInternal)
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

// TestBackwardCompatibility_ExistingBehavior verifies that all existing LB types
// continue to route to their original code paths.
func TestBackwardCompatibility_ExistingBehavior(t *testing.T) {
	tests := []struct {
		name         string
		lbType       infrav1.LoadBalancerType
		wantGlobal   bool
		wantRegional bool
		description  string
	}{
		{
			name:         "External routes to GLOBAL external LB (original)",
			lbType:       infrav1.External,
			wantGlobal:   true,
			wantRegional: false,
			description:  "Uses createExternalLoadBalancer() - original global path",
		},
		{
			name:         "InternalExternal routes to GLOBAL external LB (original)",
			lbType:       infrav1.InternalExternal,
			wantGlobal:   true,
			wantRegional: false,
			description:  "Uses createExternalLoadBalancer() - original global path",
		},
		{
			name:         "RegionalExternal routes to REGIONAL external LB (new)",
			lbType:       infrav1.RegionalExternal,
			wantGlobal:   false,
			wantRegional: true,
			description:  "Uses createRegionalExternalLoadBalancer() - NEW regional path for GCD",
		},
		{
			name:         "RegionalInternalExternal routes to REGIONAL external LB (new)",
			lbType:       infrav1.RegionalInternalExternal,
			wantGlobal:   false,
			wantRegional: true,
			description:  "Uses createRegionalExternalLoadBalancer() - NEW regional path for GCD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isRegional := isRegionalExternalLoadBalancer(tt.lbType)

			// If regional, should NOT go to global path
			if tt.wantRegional && !isRegional {
				t.Errorf("Expected regional routing, but isRegionalExternalLoadBalancer() = false")
			}

			// If global, should NOT go to regional path
			if tt.wantGlobal && isRegional {
				t.Errorf("Expected global routing, but isRegionalExternalLoadBalancer() = true")
			}

			t.Logf("✅ %s", tt.description)
		})
	}
}

// TestBackwardCompatibility_DefaultNaming verifies that default naming behavior
// remains unchanged.
func TestBackwardCompatibility_DefaultNaming(t *testing.T) {
	tests := []struct {
		name     string
		lbSpec   infrav1.LoadBalancerSpec
		wantName string
	}{
		{
			name:     "External LB defaults to apiserver name",
			lbSpec:   infrav1.LoadBalancerSpec{},
			wantName: infrav1.APIServerRoleTagValue,
		},
		{
			name: "Internal LB defaults to api-internal name",
			lbSpec: infrav1.LoadBalancerSpec{
				InternalLoadBalancer: &infrav1.LoadBalancer{},
			},
			wantName: infrav1.InternalRoleTagValue,
		},
		{
			name: "Custom external LB name is respected",
			lbSpec: infrav1.LoadBalancerSpec{
				ExternalLoadBalancer: &infrav1.LoadBalancer{
					Name: ptr.To("custom-lb"),
				},
			},
			wantName: "custom-lb",
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
			var gotName string
			// Determine which function to call based on which LB is configured
			if tt.lbSpec.InternalLoadBalancer != nil {
				gotName = getInternalLoadBalancerName(tt.lbSpec)
			} else {
				gotName = getExternalLoadBalancerName(tt.lbSpec)
			}

			if gotName != tt.wantName {
				t.Errorf("Name = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}
