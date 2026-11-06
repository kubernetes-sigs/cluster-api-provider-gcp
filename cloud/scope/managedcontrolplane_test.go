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

package scope

import (
	"testing"

	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/location"
)

func TestManagedControlPlaneScope_ValidateLocation(t *testing.T) {
	tests := []struct {
		name             string
		resolvedLocation location.ResolvedLocation
		region           string
		enableAutopilot  bool
		wantReason       string
	}{
		{
			name: "regional",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-central1",
				Regional: true,
			},
			region:          "us-central1",
			enableAutopilot: false,
			wantReason:      "",
		},
		{
			name: "zonal",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-central1",
				Regional: false,
				Zone:     new("us-central1-a"),
			},
			region:          "us-central1",
			enableAutopilot: false,
			wantReason:      "",
		},
		{
			name: "regional with autopilot",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-central1",
				Regional: true,
			},
			region:          "us-central1",
			enableAutopilot: true,
			wantReason:      "",
		},
		{
			name: "regional mismatch",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-east1",
				Regional: true,
			},
			region:          "us-central1",
			enableAutopilot: true,
			wantReason:      v1beta1.GKEControlPlaneRegionMismatchReason,
		},
		{
			name: "zonal plus autopilot",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-central1",
				Regional: false,
				Zone:     new("us-central1-a"),
			},
			region:          "us-central1",
			enableAutopilot: true,
			wantReason:      v1beta1.GKEControlPlaneAutopilotRequiresRegionalReason,
		},
		{
			name: "zonal plus autopilot plus mismatch",
			resolvedLocation: location.ResolvedLocation{
				Region:   "us-east1",
				Regional: false,
				Zone:     new("us-east1-a"),
			},
			region:          "us-central1",
			enableAutopilot: true,
			wantReason:      v1beta1.GKEControlPlaneRegionMismatchReason,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ManagedControlPlaneScope{
				ResolvedLocation: tt.resolvedLocation,
				GCPManagedCluster: &v1beta1.GCPManagedCluster{Spec: v1beta1.GCPManagedClusterSpec{
					Region: tt.region,
				}},
				GCPManagedControlPlane: &v1beta1.GCPManagedControlPlane{Spec: v1beta1.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: v1beta1.GCPManagedControlPlaneClassSpec{
						EnableAutopilot: tt.enableAutopilot,
					},
				}},
			}
			gotReason, _ := s.ValidateLocation()
			if gotReason != tt.wantReason {
				t.Errorf("ValidateLocation() gotReason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}
