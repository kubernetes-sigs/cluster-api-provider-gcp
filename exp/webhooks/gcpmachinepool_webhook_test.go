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
	"sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func TestGCPMachinePoolValidatingWebhookCreate(t *testing.T) {
	tests := []struct {
		name        string
		spec        expinfrav1.GCPMachinePoolSpec
		expectError bool
	}{
		{
			name: "valid resource policy",
			spec: expinfrav1.GCPMachinePoolSpec{
				ResourcePolicies: []string{
					"https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/resourcePolicies/spread",
					"regions/us-central1/resourcePolicies/spread-2",
				},
			},
			expectError: false,
		},
		{
			name: "invalid resource policy",
			spec: expinfrav1.GCPMachinePoolSpec{
				ResourcePolicies: []string{"foo-bar"},
			},
			expectError: true,
		},
		{
			name: "GKE only reservation affinity setting is not allowed",
			spec: expinfrav1.GCPMachinePoolSpec{
				ReservationAffinity: &v1beta1.ReservationAffinity{
					ConsumeReservationType: v1beta1.ReservationConsumeTypeAnyReservationThenFail,
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mp := &expinfrav1.GCPMachinePool{
				Spec: tc.spec,
			}
			_, err := (&GCPMachinePool{}).ValidateCreate(t.Context(), mp)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestGCPMachinePoolValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		spec        expinfrav1.GCPMachinePoolSpec
		expectError bool
	}{
		{
			name: "valid resource policy",
			spec: expinfrav1.GCPMachinePoolSpec{
				ResourcePolicies: []string{
					"https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/resourcePolicies/spread",
					"regions/us-central1/resourcePolicies/spread-2",
				},
			},
			expectError: false,
		},
		{
			name: "invalid resource policy",
			spec: expinfrav1.GCPMachinePoolSpec{
				ResourcePolicies: []string{"foo-bar"},
			},
			expectError: true,
		},
		{
			name: "changing reservation affinity is allowed",
			spec: expinfrav1.GCPMachinePoolSpec{
				ReservationAffinity: &v1beta1.ReservationAffinity{
					ConsumeReservationType: v1beta1.ReservationConsumeTypeNoReservation,
				},
			},
			expectError: false,
		},
		{
			name: "GKE only reservation affinity setting is not allowed",
			spec: expinfrav1.GCPMachinePoolSpec{
				ReservationAffinity: &v1beta1.ReservationAffinity{
					ConsumeReservationType: v1beta1.ReservationConsumeTypeAnyReservationThenFail,
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMP := &expinfrav1.GCPMachinePool{
				Spec: tc.spec,
			}
			oldMP := &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					ResourcePolicies: []string{
						"https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/resourcePolicies/spread",
						"regions/us-central1/resourcePolicies/spread-2",
					},
					ReservationAffinity: &v1beta1.ReservationAffinity{
						ConsumeReservationType: v1beta1.ReservationConsumeTypeAnyReservation,
					},
				},
			}

			warn, err := (&GCPMachinePool{}).ValidateUpdate(t.Context(), oldMP, newMP)

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
