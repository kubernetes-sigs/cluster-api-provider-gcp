/*
Copyright The Kubernetes Authors.

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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

func TestGCPManagedControlPlaneIsDeleted(t *testing.T) {
	tests := []struct {
		name         string
		controlPlane *GCPManagedControlPlane
		want         bool
	}{
		{
			name:         "GKEControlPlaneDeletingCondition not yet set",
			controlPlane: &GCPManagedControlPlane{},
			want:         false,
		},
		{
			name: "still deleting",
			controlPlane: &GCPManagedControlPlane{
				Status: GCPManagedControlPlaneStatus{
					Conditions: clusterv1beta1.Conditions{
						{
							Type:   GKEControlPlaneDeletingCondition,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "deleted",
			controlPlane: &GCPManagedControlPlane{
				Status: GCPManagedControlPlaneStatus{
					Conditions: clusterv1beta1.Conditions{
						{
							Type:   GKEControlPlaneDeletingCondition,
							Status: corev1.ConditionFalse,
							Reason: GKEControlPlaneDeletedReason,
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tc.controlPlane.IsDeleted()).To(Equal(tc.want))
		})
	}
}
