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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestValidateLoadBalancerSpec(t *testing.T) {
	tests := []struct {
		name         string
		spec         infrav1.LoadBalancerSpec
		wantWarnings int
	}{
		{
			name: "empty spec emits no warning",
			spec: infrav1.LoadBalancerSpec{},
		},
		{
			name: "External type emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.External),
			},
		},
		{
			name: "RegionalExternal type emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalExternal),
			},
		},
		{
			name: "ExternalLoadBalancerConfig with Internal LB emits warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.Internal),
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					Name: ptr.To("ignored"),
				},
			},
			wantWarnings: 1,
		},
		{
			name: "ExternalLoadBalancerConfig with External LB emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.External),
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					Name: ptr.To("api-lb"),
				},
			},
		},
		{
			name: "ExternalLoadBalancerConfig with RegionalExternal LB emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalExternal),
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					Name: ptr.To("api-lb"),
				},
			},
		},
		{
			name: "ExternalLoadBalancerConfig with InternalExternal LB emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.InternalExternal),
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					Name: ptr.To("api-lb"),
				},
			},
		},
		{
			name: "ExternalLoadBalancerConfig with RegionalInternalExternal LB emits no warning",
			spec: infrav1.LoadBalancerSpec{
				LoadBalancerType: ptr.To(infrav1.RegionalInternalExternal),
				ExternalLoadBalancerConfig: &infrav1.ExternalLoadBalancer{
					Name: ptr.To("api-lb"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := validateLoadBalancerSpec(&tt.spec, field.NewPath("spec", "loadBalancer"))
			if got := len(warnings); got != tt.wantWarnings {
				t.Fatalf("validateLoadBalancerSpec() warnings count = %d, want %d (warnings=%v)", got, tt.wantWarnings, warnings)
			}
			for _, w := range warnings {
				if !strings.Contains(w, "spec.loadBalancer.externalLoadBalancerConfig") {
					t.Errorf("warning %q missing expected field path prefix", w)
				}
			}
		})
	}
}
