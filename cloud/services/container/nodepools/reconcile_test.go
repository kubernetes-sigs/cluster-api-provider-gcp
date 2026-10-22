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
	"context"
	"net"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

var surgeUpdateStrategy = containerpb.NodePoolUpdateStrategy_SURGE

func init() {
	_ = infrav1exp.AddToScheme(scheme.Scheme)
}

func newTestService(machinePool *infrav1exp.GCPManagedMachinePool, mp *clusterv1.MachinePool, controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedMachinePoolScope)
	s.GCPManagedMachinePool = machinePool
	s.MachinePool = mp
	s.GCPManagedControlPlane = controlPlane
	return &Service{scope: s}
}

func newTestNodePoolService(machinePool *infrav1exp.GCPManagedMachinePool, controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedMachinePoolScope)
	s.GCPManagedMachinePool = machinePool
	s.GCPManagedControlPlane = controlPlane
	s.MachinePool = &clusterv1.MachinePool{
		Spec: clusterv1.MachinePoolSpec{
			Replicas: ptr.To(int32(1)),
		},
	}
	return &Service{scope: s}
}

func defaultControlPlane() *infrav1exp.GCPManagedControlPlane {
	return &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
				Project:     "test-project",
				Location:    "us-central1-a",
				ClusterName: "test-cluster",
			},
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
			name: "diff on instance type triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						InstanceType: ptr.To("n1-standard-4"),
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					MachineType: "n1-standard-2",
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if req.GetMachineType() != "n1-standard-4" {
					t.Errorf("expected machine type n1-standard-4, got %v", req.GetMachineType())
				}
			},
		},
		{
			name: "no diff on instance type when values match",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						InstanceType: ptr.To("n1-standard-4"),
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					MachineType: "n1-standard-4",
				},
			},
			wantNeedUpdate: false,
		},
		{
			name:        "no update when instance type is unset",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					MachineType: "n1-standard-4",
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "diff on upgrade settings triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						UpgradeSettings: &infrav1exp.NodePoolUpgradeSettings{
							Strategy:       ptr.To("SURGE"),
							MaxSurge:       ptr.To(int32(1)),
							MaxUnavailable: ptr.To(int32(0)),
						},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateNodePoolRequest) {
				t.Helper()
				if req.GetUpgradeSettings().GetMaxSurge() != 1 {
					t.Errorf("expected upgrade settings max surge 1, got %v", req.GetUpgradeSettings().GetMaxSurge())
				}
				if req.GetUpgradeSettings().GetStrategy() != containerpb.NodePoolUpdateStrategy_SURGE {
					t.Errorf("expected upgrade settings strategy SURGE, got %v", req.GetUpgradeSettings().GetStrategy())
				}
			},
		},
		{
			name: "update not needed if defaults align",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						UpgradeSettings: &infrav1exp.NodePoolUpgradeSettings{
							Strategy: ptr.To("SURGE"),
						},
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				UpgradeSettings: &containerpb.NodePool_UpgradeSettings{
					MaxSurge:       1,
					MaxUnavailable: 0,
					Strategy:       &surgeUpdateStrategy,
				},
			},
			wantNeedUpdate: false,
		},
		{
			name:        "no update when upgrade settings unset",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
				UpgradeSettings: &containerpb.NodePool_UpgradeSettings{
					MaxSurge: 1,
				},
			},
			wantNeedUpdate: false,
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

func TestCheckDiffAndPrepareUpdateConfig_WorkloadMetadataMode(t *testing.T) {
	controlPlane := &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
				Project:     "test-project",
				Location:    "us-central1-a",
				ClusterName: "test-cluster",
			},
		},
	}

	tests := []struct {
		name                 string
		workloadMetadataMode *infrav1exp.WorkloadMetadataMode
		existingNodePool     *containerpb.NodePool
		wantMetadataMode     *containerpb.WorkloadMetadataConfig_Mode
	}{
		{
			name:                 "nil mode does not trigger update even when existing has GKE_METADATA",
			workloadMetadataMode: nil,
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					WorkloadMetadataConfig: &containerpb.WorkloadMetadataConfig{
						Mode: containerpb.WorkloadMetadataConfig_GKE_METADATA,
					},
				},
			},
			wantMetadataMode: nil,
		},
		{
			name:                 "GKE_METADATA mode triggers update when existing is unset",
			workloadMetadataMode: ptr.To(infrav1exp.WorkloadMetadataModeGKEMetadata),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantMetadataMode: ptr.To(containerpb.WorkloadMetadataConfig_GKE_METADATA),
		},
		{
			name:                 "GCE_METADATA mode triggers update when existing has GKE_METADATA",
			workloadMetadataMode: ptr.To(infrav1exp.WorkloadMetadataModeGCEMetadata),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					WorkloadMetadataConfig: &containerpb.WorkloadMetadataConfig{
						Mode: containerpb.WorkloadMetadataConfig_GKE_METADATA,
					},
				},
			},
			wantMetadataMode: ptr.To(containerpb.WorkloadMetadataConfig_GCE_METADATA),
		},
		{
			name:                 "no update when desired GKE_METADATA matches existing",
			workloadMetadataMode: ptr.To(infrav1exp.WorkloadMetadataModeGKEMetadata),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					WorkloadMetadataConfig: &containerpb.WorkloadMetadataConfig{
						Mode: containerpb.WorkloadMetadataConfig_GKE_METADATA,
					},
				},
			},
			wantMetadataMode: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machinePool := &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						NodePoolName: "test-pool",
						NodeSecurity: infrav1exp.NodeSecurityConfig{
							WorkloadMetadataMode: tt.workloadMetadataMode,
						},
					},
				},
			}

			svc := newTestNodePoolService(machinePool, controlPlane)
			_, updateReq := svc.checkDiffAndPrepareUpdateConfig(tt.existingNodePool, nil)

			if tt.wantMetadataMode == nil {
				if updateReq.GetWorkloadMetadataConfig() != nil {
					t.Errorf("expected nil WorkloadMetadataConfig, got %v", updateReq.GetWorkloadMetadataConfig())
				}
			} else if mode := updateReq.GetWorkloadMetadataConfig().GetMode(); mode != *tt.wantMetadataMode {
				t.Errorf("expected WorkloadMetadataConfig.Mode %v, got %v", *tt.wantMetadataMode, mode)
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
			name:         "no diff when replica count matches",
			machinePool:  defaultGCPManagedMachinePool(),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "diff detected when replica count increases",
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
			name:         "diff detected when replica count decreases",
			machinePool:  defaultGCPManagedMachinePool(),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(1))}},
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
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
					},
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
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
					},
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
			if tt.wantNeedUpdate && req != nil && req.GetNodeCount() != tt.wantNodeCount {
				t.Errorf("checkDiffAndPrepareUpdateSize() NodeCount = %v, want %v", req.GetNodeCount(), tt.wantNodeCount)
			}
		})
	}
}

type fakeClusterManager struct {
	containerpb.UnimplementedClusterManagerServer
	nodePool *containerpb.NodePool
}

func (f *fakeClusterManager) GetNodePool(_ context.Context, _ *containerpb.GetNodePoolRequest) (*containerpb.NodePool, error) {
	return f.nodePool, nil
}

// newReconcileTestService returns a Service backed by a fake GKE API that always returns nodePool.
// The machine pool starts as ready, so tests can check that a reconcile marks it as not ready.
func newReconcileTestService(t *testing.T, nodePool *containerpb.NodePool) *Service {
	t.Helper()
	ctx := context.TODO()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	containerpb.RegisterClusterManagerServer(server, &fakeClusterManager{nodePool: nodePool})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	mcClient, err := container.NewClusterManagerClient(ctx,
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		t.Fatal(err)
	}
	// The compute clients are never called because the node pool has no instance groups.
	migClient, err := compute.NewInstanceGroupManagersRESTClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	templatesClient, err := compute.NewInstanceTemplatesRESTClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}

	gcpMachinePool := &infrav1exp.GCPManagedMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "test-pool"}}
	gcpMachinePool.Status.Ready = true
	v1beta1conditions.MarkTrue(gcpMachinePool, clusterv1beta1.ReadyCondition)
	v1beta2conditions.Set(gcpMachinePool, metav1.Condition{
		Type:   infrav1exp.GCPManagedMachinePoolReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1exp.GKEMachinePoolCreatedReason,
	})

	s, err := scope.NewManagedMachinePoolScope(ctx, scope.ManagedMachinePoolScopeParams{
		Client:                      fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		Cluster:                     &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}},
		MachinePool:                 defaultMachinePool(),
		GCPManagedCluster:           &infrav1exp.GCPManagedCluster{},
		GCPManagedControlPlane:      defaultControlPlane(),
		GCPManagedMachinePool:       gcpMachinePool,
		ManagedClusterClient:        mcClient,
		InstanceGroupManagersClient: migClient,
		InstanceTemplatesClient:     templatesClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })

	return New(s)
}

func TestService_Reconcile(t *testing.T) {
	tests := []struct {
		name        string
		nodePool    *containerpb.NodePool
		wantErr     bool
		wantReason  string
		wantMessage string
	}{
		{
			name: "node pool in error",
			nodePool: &containerpb.NodePool{
				Status:     containerpb.NodePool_ERROR,
				Conditions: []*containerpb.StatusCondition{{Message: "quota exceeded"}},
			},
			wantReason:  infrav1exp.GKEMachinePoolErrorReason,
			wantMessage: "quota exceeded",
		},
		{
			name:       "node pool running with error",
			nodePool:   &containerpb.NodePool{Status: containerpb.NodePool_RUNNING_WITH_ERROR},
			wantReason: infrav1exp.GKEMachinePoolErrorReason,
		},
		{
			name:       "node pool provisioning",
			nodePool:   &containerpb.NodePool{Status: containerpb.NodePool_PROVISIONING},
			wantReason: infrav1exp.GKEMachinePoolCreatingReason,
		},
		{
			name:       "node pool stopping",
			nodePool:   &containerpb.NodePool{Status: containerpb.NodePool_STOPPING},
			wantReason: infrav1exp.GKEMachinePoolDeletingReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			svc := newReconcileTestService(t, tt.nodePool)
			_, err := svc.Reconcile(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			machinePool := svc.scope.GCPManagedMachinePool
			if machinePool.Status.Ready {
				t.Errorf("status.ready = true, want false")
			}

			for _, conditionType := range []clusterv1beta1.ConditionType{clusterv1beta1.ReadyCondition, infrav1exp.GKEMachinePoolReadyCondition} {
				c := v1beta1conditions.Get(machinePool, conditionType)
				if c == nil {
					t.Errorf("v1beta1 condition %s not set", conditionType)
					continue
				}
				if c.Status != corev1.ConditionFalse || c.Reason != tt.wantReason || c.Message != tt.wantMessage {
					t.Errorf("v1beta1 condition %s = %s/%s/%q, want False/%s/%q", conditionType, c.Status, c.Reason, c.Message, tt.wantReason, tt.wantMessage)
				}
			}

			for _, conditionType := range []string{infrav1exp.GCPManagedMachinePoolReadyCondition, infrav1exp.GCPManagedMachinePoolGKEMachinePoolReadyCondition} {
				c := v1beta2conditions.Get(machinePool, conditionType)
				if c == nil {
					t.Errorf("v1beta2 condition %s not set", conditionType)
					continue
				}
				if c.Status != metav1.ConditionFalse || c.Reason != tt.wantReason || c.Message != tt.wantMessage {
					t.Errorf("v1beta2 condition %s = %s/%s/%q, want False/%s/%q", conditionType, c.Status, c.Reason, c.Message, tt.wantReason, tt.wantMessage)
				}
			}
		})
	}
}
