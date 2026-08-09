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

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func TestManagedControlPlaneScope_FleetProject(t *testing.T) {
	tests := []struct {
		name         string
		controlPlane *infrav1exp.GCPManagedControlPlane
		want         string
	}{
		{
			name: "fleet unset defaults to cluster's own project",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project: "cluster-project",
					},
				},
			},
			want: "cluster-project",
		},
		{
			name: "fleet set but project unset defaults to cluster's own project",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project: "cluster-project",
						Fleet:   &infrav1exp.Fleet{},
					},
				},
			},
			want: "cluster-project",
		},
		{
			name: "fleet project set overrides cluster's own project",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project: "cluster-project",
						Fleet:   &infrav1exp.Fleet{Project: ptr.To("fleet-host-project")},
					},
				},
			},
			want: "fleet-host-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ManagedControlPlaneScope{GCPManagedControlPlane: tt.controlPlane}
			assert.Equal(t, tt.want, s.FleetProject())
		})
	}
}
