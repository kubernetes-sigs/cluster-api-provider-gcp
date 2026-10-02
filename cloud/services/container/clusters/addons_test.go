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

package clusters

import (
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1/addons"
)

func managedPool(name string, spec infrav1exp.GCPManagedMachinePoolClassSpec) infrav1exp.GCPManagedMachinePool {
	return infrav1exp.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       infrav1exp.GCPManagedMachinePoolSpec{GCPManagedMachinePoolClassSpec: spec},
	}
}

// TestMergeNodePoolStates covers the precedence the add-on constraints depend on: what a pool asks for
// wins, what GKE resolved fills the gaps, and a field neither settles stays empty so that a constraint
// skips the pool rather than judging it on a guess.
func TestMergeNodePoolStates(t *testing.T) {
	tests := []struct {
		name     string
		pools    []infrav1exp.GCPManagedMachinePool
		existing *containerpb.Cluster
		want     []addons.NodePoolState
	}{
		{
			name: "what the pool asks for is used",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-0", infrav1exp.GCPManagedMachinePoolClassSpec{
					MachineType: ptr.To("n2-standard-4"),
					ImageType:   ptr.To("cos_containerd"),
				}),
			},
			want: []addons.NodePoolState{
				{Name: "pool-0", MachineType: "n2-standard-4", ImageType: "cos_containerd"},
			},
		},
		{
			name: "instanceType wins over machineType",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-0", infrav1exp.GCPManagedMachinePoolClassSpec{
					MachineType:  ptr.To("n2-standard-4"),
					InstanceType: ptr.To("t2a-standard-4"),
				}),
			},
			want: []addons.NodePoolState{{Name: "pool-0", MachineType: "t2a-standard-4"}},
		},
		{
			name: "what GKE resolved fills in what the pool left alone",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-0", infrav1exp.GCPManagedMachinePoolClassSpec{}),
			},
			existing: &containerpb.Cluster{NodePools: []*containerpb.NodePool{{
				Name: "pool-0",
				Config: &containerpb.NodeConfig{
					MachineType:   "e2-medium",
					ImageType:     "COS_CONTAINERD",
					SandboxConfig: &containerpb.SandboxConfig{Type: containerpb.SandboxConfig_GVISOR},
				},
			}}},
			want: []addons.NodePoolState{
				{Name: "pool-0", MachineType: "e2-medium", ImageType: "COS_CONTAINERD", SandboxType: "GVISOR"},
			},
		},
		{
			name: "the pool still wins over what GKE resolved",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-0", infrav1exp.GCPManagedMachinePoolClassSpec{MachineType: ptr.To("n2-standard-4")}),
			},
			existing: &containerpb.Cluster{NodePools: []*containerpb.NodePool{{
				Name:   "pool-0",
				Config: &containerpb.NodeConfig{MachineType: "e2-medium"},
			}}},
			want: []addons.NodePoolState{{Name: "pool-0", MachineType: "n2-standard-4"}},
		},
		{
			name: "nothing to fall back to leaves the field empty",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-0", infrav1exp.GCPManagedMachinePoolClassSpec{}),
			},
			want: []addons.NodePoolState{{Name: "pool-0"}},
		},
		{
			name: "a pool is matched to GKE by its node pool name, not its object name",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("object-name", infrav1exp.GCPManagedMachinePoolClassSpec{NodePoolName: "gke-name"}),
			},
			existing: &containerpb.Cluster{NodePools: []*containerpb.NodePool{{
				Name:   "gke-name",
				Config: &containerpb.NodeConfig{MachineType: "e2-medium"},
			}}},
			want: []addons.NodePoolState{{Name: "gke-name", MachineType: "e2-medium"}},
		},
		{
			name: "a pool GKE doesn't know about yet",
			pools: []infrav1exp.GCPManagedMachinePool{
				managedPool("pool-1", infrav1exp.GCPManagedMachinePoolClassSpec{MachineType: ptr.To("n2-standard-4")}),
			},
			existing: &containerpb.Cluster{NodePools: []*containerpb.NodePool{{
				Name:   "pool-0",
				Config: &containerpb.NodeConfig{MachineType: "e2-medium"},
			}}},
			want: []addons.NodePoolState{{Name: "pool-1", MachineType: "n2-standard-4"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mergeNodePoolStates(test.pools, test.existing)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("mergeNodePoolStates() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
