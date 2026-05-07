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

package nodepools

import (
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func newTestService(machinePool *infrav1exp.GCPManagedMachinePool, mp *clusterv1.MachinePool, controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedMachinePoolScope)
	s.GCPManagedMachinePool = machinePool
	s.MachinePool = mp
	s.GCPManagedControlPlane = controlPlane
	return &Service{scope: s}
}

func defaultControlPlane() *infrav1exp.GCPManagedControlPlane {
	return &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
				Project:  "test-project",
				Location: "us-central1-a",
			},
			ClusterName: "test-cluster",
		},
	}
}

func defaultMachinePool() *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		Spec: clusterv1.MachinePoolSpec{
			Replicas: ptr.To(int32(3)),
		},
	}
}

func defaultGCPManagedMachinePool() *infrav1exp.GCPManagedMachinePool {
	return &infrav1exp.GCPManagedMachinePool{}
}

func TestCheckDiffAndPrepareUpdateConfig(t *testing.T) {
	tests := []struct {
		name                   string
		machinePool            *infrav1exp.GCPManagedMachinePool
		mp                     *clusterv1.MachinePool
		existingNodePool       *containerpb.NodePool
		existingTemplateLabels map[string]string
		wantNeedUpdate         bool
		validateUpdateFunc     func(t *testing.T, req *containerpb.UpdateNodePoolRequest)
	}{
		{
			name:        "no diff when everything matches",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantNeedUpdate: false,
		},
		{
			name: "no diff when desired resource labels already present in instance template",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						AdditionalLabels: infrav1.Labels{"user-label": "value"},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			// Template already has both the user label and the CAPG ownership label.
			existingTemplateLabels: map[string]string{
				"user-label":                "value",
				"capg-cluster-test-cluster": "owned",
				"goog-gke-node":             "",
			},
			wantNeedUpdate: false,
		},
		{
			name: "diff on resource labels triggers update when label missing from template",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						AdditionalLabels: infrav1.Labels{"user-label": "value"},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			// Template is missing the desired user-label.
			existingTemplateLabels: map[string]string{
				"capg-cluster-test-cluster": "owned",
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if req.GetResourceLabels().GetLabels()["user-label"] != "value" {
					t.Errorf("expected resource label user-label=value, got %v", req.GetResourceLabels().GetLabels())
				}
			},
		},
		{
			name: "no resource label update when template labels unavailable",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						AdditionalLabels: infrav1.Labels{"user-label": "value"},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			existingTemplateLabels: nil,
			wantNeedUpdate:         false,
		},
		{
			name: "diff on Kubernetes labels triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						KubernetesLabels: infrav1.Labels{"env": "prod"},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					Labels: map[string]string{"env": "staging"},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if req.GetLabels().GetLabels()["env"] != "prod" {
					t.Errorf("expected label env=prod, got %v", req.GetLabels().GetLabels())
				}
			},
		},
		{
			name: "no diff on image type when case differs (case-insensitive match)",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						ImageType: ptr.To("cos_containerd"),
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					ImageType: "COS_CONTAINERD",
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "diff on image type triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						ImageType: ptr.To("ubuntu_containerd"),
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					ImageType: "COS_CONTAINERD",
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if req.GetImageType() != "ubuntu_containerd" {
					t.Errorf("expected image type ubuntu_containerd, got %v", req.GetImageType())
				}
			},
		},
		{
			name: "diff on network tags triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						NodeNetwork: infrav1exp.NodeNetworkConfig{
							Tags: []string{"tag-a", "tag-b"},
						},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					Tags: []string{"tag-a"},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if len(req.GetTags().GetTags()) != 2 {
					t.Errorf("expected 2 tags, got %v", req.GetTags().GetTags())
				}
			},
		},
		{
			name: "diff on node locations triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						NodeLocations: []string{"us-central1-a", "us-central1-b"},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config:    &containerpb.NodeConfig{},
				Locations: []string{"us-central1-a"},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if len(req.GetLocations()) != 2 {
					t.Errorf("expected 2 locations, got %v", req.GetLocations())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(tt.machinePool, tt.mp, defaultControlPlane())
			needUpdate, req := svc.checkDiffAndPrepareUpdateConfig(tt.existingNodePool, tt.existingTemplateLabels)
			if needUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdateConfig() needUpdate = %v, want %v", needUpdate, tt.wantNeedUpdate)
			}
			if tt.validateUpdateFunc != nil {
				tt.validateUpdateFunc(t, req)
			}
		})
	}
}

func TestCheckDiffAndPrepareUpdateSize(t *testing.T) {
	tests := []struct {
		name             string
		machinePool      *infrav1exp.GCPManagedMachinePool
		mp               *clusterv1.MachinePool
		controlPlane     *infrav1exp.GCPManagedControlPlane
		existingNodePool *containerpb.NodePool
		wantNeedUpdate   bool
		wantNodeCount    int32
	}{
		{
			name:        "no diff when replica count matches",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: false,
		},
		{
			name:        "diff detected when replica count increases",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(5))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  5,
		},
		{
			name:        "diff detected when replica count decreases",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(1))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  1,
		},
		{
			name: "autoscaling enabled suppresses size update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						Scaling: &infrav1exp.NodePoolAutoScaling{
							MinCount: ptr.To(int32(1)),
							MaxCount: ptr.To(int32(10)),
						},
					},
				},
			},
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(5))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "nil Scaling spec does not suppress size update",
			machinePool:  defaultGCPManagedMachinePool(),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(5))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  5,
		},
		{
			name:        "regional cluster divides replicas by zone count",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(6))}},
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:  "test-project",
						Location: "us-central1",
					},
					ClusterName: "test-cluster",
				},
			},
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 2,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: false,
		},
		{
			name:        "regional cluster detects diff after division",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(9))}},
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:  "test-project",
						Location: "us-central1",
					},
					ClusterName: "test-cluster",
				},
			},
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 2,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(tt.machinePool, tt.mp, tt.controlPlane)
			needUpdate, req := svc.checkDiffAndPrepareUpdateSize(tt.existingNodePool)
			if needUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdateSize() needUpdate = %v, want %v", needUpdate, tt.wantNeedUpdate)
			}
			if tt.wantNeedUpdate && req != nil && req.NodeCount != tt.wantNodeCount {
				t.Errorf("checkDiffAndPrepareUpdateSize() NodeCount = %v, want %v", req.NodeCount, tt.wantNodeCount)
			}
		})
	}
}
