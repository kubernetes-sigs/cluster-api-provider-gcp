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
	"fmt"
	"net"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	. "github.com/onsi/gomega"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-gcp/util/location"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

var surgeUpdateStrategy = containerpb.NodePoolUpdateStrategy_SURGE

func init() {
	_ = infrav1exp.AddToScheme(scheme.Scheme)
}

func newTestService(machinePool *infrav1exp.GCPManagedMachinePool, mp *clusterv1.MachinePool, controlPlane *infrav1exp.GCPManagedControlPlane, location location.ResolvedLocation) *Service {
	s := new(scope.ManagedMachinePoolScope)
	s.GCPManagedMachinePool = machinePool
	s.MachinePool = mp
	s.GCPManagedControlPlane = controlPlane
	s.ResolvedLocation = location
	return &Service{scope: s}
}

func newTestNodePoolService(machinePool *infrav1exp.GCPManagedMachinePool, controlPlane *infrav1exp.GCPManagedControlPlane, location location.ResolvedLocation) *Service {
	s := new(scope.ManagedMachinePoolScope)
	s.GCPManagedMachinePool = machinePool
	s.GCPManagedControlPlane = controlPlane
	s.ResolvedLocation = location
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

var zonalLocation = location.ResolvedLocation{
	Region:   "us-central1",
	Zone:     new("us-central1-a"),
	Regional: false,
}

var regionalLocation = location.ResolvedLocation{
	Region:   "us-central1",
	Zone:     nil,
	Regional: true,
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

func createGCPManagedMachinePoolWithProviderList(listLen int) *infrav1exp.GCPManagedMachinePool {
	return &infrav1exp.GCPManagedMachinePool{
		Spec: infrav1exp.GCPManagedMachinePoolSpec{
			ProviderIDList: createProviderIDList(listLen),
		},
	}
}

func createProviderIDList(length int) []string {
	prefixFormat := "gce://my-project/us-central1-%c/node-%d"
	var providerIDList []string
	for i := 1; i < length+1; i++ {
		providerIDList = append(providerIDList, fmt.Sprintf(prefixFormat, 'a'+rune(i-1), i))
	}
	return providerIDList
}

func TestCheckDiffAndPrepareUpdateConfig(t *testing.T) {
	tests := []struct {
		name                   string
		location               location.ResolvedLocation
		machinePool            *infrav1exp.GCPManagedMachinePool
		mp                     *clusterv1.MachinePool
		existingNodePool       *containerpb.NodePool
		existingTemplateLabels map[string]string
		wantNeedUpdate         bool
		validateUpdateFunc     func(t *testing.T, req *containerpb.UpdateNodePoolRequest)
	}{
		{
			name:        "no diff when everything matches",
			location:    zonalLocation,
			machinePool: defaultGCPManagedMachinePool(),
			mp:          defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantNeedUpdate: false,
		},
		{
			name:     "no diff when desired resource labels already present in instance template",
			location: zonalLocation,
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
			name:     "diff on resource labels triggers update when label missing from template",
			location: zonalLocation,
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
			name:     "no resource label update when template labels unavailable",
			location: zonalLocation,
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
			name:     "diff on Kubernetes labels triggers update",
			location: zonalLocation,
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
			name:     "no diff on image type when case differs (case-insensitive match)",
			location: zonalLocation,
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
			name:     "diff on image type triggers update",
			location: zonalLocation,
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
			name:     "diff on instance type triggers update",
			location: zonalLocation,
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
			name:     "no diff on instance type when values match",
			location: zonalLocation,
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
			location:    zonalLocation,
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
			name:     "diff on upgrade settings triggers update",
			location: zonalLocation,
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
			name:     "update not needed if defaults align",
			location: zonalLocation,
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
			location:    zonalLocation,
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
			name:     "diff on network tags triggers update",
			location: zonalLocation,
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
			name:     "diff on node locations triggers update",
			location: regionalLocation,
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
		{
			// GKE omits TaintConfig entirely for a pool that has never set one, so a spec that
			// leaves the field unset must not register a diff on every reconcile.
			name:        "no diff when architecture taint behavior is unset",
			machinePool: defaultGCPManagedMachinePool(),
			mp:          defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantNeedUpdate: false,
		},
		{
			name: "diff on architecture taint behavior triggers update",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						ArchitectureTaintBehavior: ptr.To(infrav1exp.ArchitectureTaintBehaviorNone),
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
				if got := req.GetTaintConfig().GetArchitectureTaintBehavior(); got != containerpb.TaintConfig_NONE {
					t.Errorf("expected architecture taint behavior NONE, got %v", got)
				}
			},
		},
		{
			name: "no diff when architecture taint behavior already matches",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						ArchitectureTaintBehavior: ptr.To(infrav1exp.ArchitectureTaintBehaviorNone),
					},
				},
			},
			mp: defaultMachinePool(),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{
					TaintConfig: &containerpb.TaintConfig{
						ArchitectureTaintBehavior: containerpb.TaintConfig_NONE.Enum(),
					},
				},
			},
			wantNeedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(tt.machinePool, tt.mp, defaultControlPlane(), tt.location)
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
		location             location.ResolvedLocation
		workloadMetadataMode *infrav1exp.WorkloadMetadataMode
		existingNodePool     *containerpb.NodePool
		wantMetadataMode     *containerpb.WorkloadMetadataConfig_Mode
	}{
		{
			name:                 "nil mode does not trigger update even when existing has GKE_METADATA",
			location:             zonalLocation,
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
			location:             zonalLocation,
			workloadMetadataMode: ptr.To(infrav1exp.WorkloadMetadataModeGKEMetadata),
			existingNodePool: &containerpb.NodePool{
				Config: &containerpb.NodeConfig{},
			},
			wantMetadataMode: ptr.To(containerpb.WorkloadMetadataConfig_GKE_METADATA),
		},
		{
			name:                 "GCE_METADATA mode triggers update when existing has GKE_METADATA",
			location:             zonalLocation,
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
			location:             zonalLocation,
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

			svc := newTestNodePoolService(machinePool, controlPlane, tt.location)
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
		location         location.ResolvedLocation
		machinePool      *infrav1exp.GCPManagedMachinePool
		mp               *clusterv1.MachinePool
		controlPlane     *infrav1exp.GCPManagedControlPlane
		existingNodePool *containerpb.NodePool
		wantNeedUpdate   bool
		wantNodeCount    int32
		wantErr          bool
	}{
		{
			name:         "no diff when replica count matches",
			location:     zonalLocation,
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
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
			location:     zonalLocation,
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
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
			location:     zonalLocation,
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
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
			name:     "autoscaling enabled suppresses size update",
			location: zonalLocation,
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
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(4))}},
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "nil Scaling spec does not suppress size update",
			location:     zonalLocation,
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
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
			location:    regionalLocation,
			machinePool: createGCPManagedMachinePoolWithProviderList(6),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(6))}},
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						ClusterName: "test-cluster",
					},
				},
			},
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 2,
			},
			wantNeedUpdate: false,
		},
		{
			name:        "zonal cluster divides replicas by zone count (should be 1)",
			location:    zonalLocation,
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						ClusterName: "test-cluster",
					},
				},
			},
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
			},
			wantNeedUpdate: false,
		},
		{
			name:        "regional cluster detects diff after division",
			location:    regionalLocation,
			machinePool: createGCPManagedMachinePoolWithProviderList(6),
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
		{
			name:        "regional with no locations but replicas that do not divide into 3",
			location:    regionalLocation,
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(4))}},
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
			},
			wantErr: true,
		},
		{
			name:        "regional with locations but replicas that do not divide the number of locations",
			location:    regionalLocation,
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(4))}},
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
				Locations:        []string{"us-central1-a", "us-central-1b", "us-central-1c"},
			},
			wantErr: true,
		},
		{
			name:        "regional with 0 replicas",
			location:    regionalLocation,
			machinePool: defaultGCPManagedMachinePool(),
			mp:          &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(0))}},
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
				InitialNodeCount: 0,
				Locations:        []string{"us-central1-a", "us-central-1b", "us-central-1c"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "autoscaler shrinks pool, then autoscaler disabled (regional)",
			machinePool:  createGCPManagedMachinePoolWithProviderList(1),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			location:     regionalLocation,
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 1,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  1,
		},
		{
			name:         "autoscaler shrinks pool, then autoscaler disabled (zonal)",
			machinePool:  createGCPManagedMachinePoolWithProviderList(1),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			location:     regionalLocation,
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 3,
				Locations:        []string{"us-central1-a"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  3,
		},
		{
			name:         "converged after autoscaler",
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			location:     regionalLocation,
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 1,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "uneven number of replicas vs zones",
			machinePool:  createGCPManagedMachinePoolWithProviderList(3),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(4))}},
			location:     regionalLocation,
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 1,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "observed is above desired",
			machinePool:  createGCPManagedMachinePoolWithProviderList(6),
			mp:           &clusterv1.MachinePool{Spec: clusterv1.MachinePoolSpec{Replicas: ptr.To(int32(3))}},
			location:     regionalLocation,
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				InitialNodeCount: 1,
				Locations:        []string{"us-central1-a", "us-central1-b", "us-central1-c"},
			},
			wantNeedUpdate: true,
			wantNodeCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			svc := newTestService(tt.machinePool, tt.mp, tt.controlPlane, tt.location)
			needUpdate, req, err := svc.checkDiffAndPrepareUpdateSize(tt.existingNodePool)

			g.Expect(err != nil).To(Equal(tt.wantErr))
			if needUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdateSize() needUpdate = %v, want %v", needUpdate, tt.wantNeedUpdate)
			}
			if tt.wantNeedUpdate && req != nil && req.GetNodeCount() != tt.wantNodeCount {
				t.Errorf("checkDiffAndPrepareUpdateSize() NodeCount = %v, want %v", req.GetNodeCount(), tt.wantNodeCount)
			}
		})
	}
}

func TestCheckDiffAndPrepareUpdateAutoscaling(t *testing.T) {
	tests := []struct {
		name             string
		machinePool      *infrav1exp.GCPManagedMachinePool
		mp               *clusterv1.MachinePool
		controlPlane     *infrav1exp.GCPManagedControlPlane
		existingNodePool *containerpb.NodePool
		wantNeedUpdate   bool
		wantEnabled      bool
	}{
		{
			name: "empty scaling block enables autoscaling",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						Scaling: &infrav1exp.NodePoolAutoScaling{},
					},
				},
			},
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{
					Enabled:           false,
					TotalMinNodeCount: 0,
					TotalMaxNodeCount: 1,
					LocationPolicy:    containerpb.NodePoolAutoscaling_BALANCED,
				},
			},
			wantNeedUpdate: true,
			wantEnabled:    true,
		},
		{
			name:         "nil scaling but existing enabled (min 0 / max 1)",
			machinePool:  defaultGCPManagedMachinePool(),
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{
					Enabled:      true,
					MinNodeCount: 0,
					MaxNodeCount: 1,
				},
			},
			wantNeedUpdate: false,
		},
		{
			name:         "nil scaling but existing {}",
			machinePool:  defaultGCPManagedMachinePool(),
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{},
			},
			wantNeedUpdate: false,
		},
		{
			name: "disabled against existing enabled",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						Scaling: &infrav1exp.NodePoolAutoScaling{
							EnableAutoscaling: new(false),
						},
					},
				},
			},
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{
					Enabled: true,
				},
			},
			wantNeedUpdate: true,
		},
		{
			name: "enabled with min/max different from existing",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						Scaling: &infrav1exp.NodePoolAutoScaling{
							EnableAutoscaling: new(true),
							MinCount:          new(int32(0)),
							MaxCount:          new(int32(4)),
						},
					},
				},
			},
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{
					Enabled:           true,
					TotalMinNodeCount: int32(7),
					TotalMaxNodeCount: int32(14),
				},
			},
			wantNeedUpdate: true,
			wantEnabled:    true,
		},
		{
			name: "enabled with equal settings",
			machinePool: &infrav1exp.GCPManagedMachinePool{
				Spec: infrav1exp.GCPManagedMachinePoolSpec{
					GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
						Scaling: &infrav1exp.NodePoolAutoScaling{
							EnableAutoscaling: new(true),
							MinCount:          new(int32(0)),
							MaxCount:          new(int32(4)),
						},
					},
				},
			},
			mp:           defaultMachinePool(),
			controlPlane: defaultControlPlane(),
			existingNodePool: &containerpb.NodePool{
				Autoscaling: &containerpb.NodePoolAutoscaling{
					Enabled:           true,
					TotalMinNodeCount: int32(0),
					TotalMaxNodeCount: int32(4),
					LocationPolicy:    containerpb.NodePoolAutoscaling_BALANCED,
				},
			},
			wantNeedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(tt.machinePool, tt.mp, tt.controlPlane, zonalLocation)
			needUpdate, req := svc.checkDiffAndPrepareUpdateAutoscaling(tt.existingNodePool)
			if needUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdateAutoscaling() needUpdate = %v, want %v", needUpdate, tt.wantNeedUpdate)
			}
			if tt.wantNeedUpdate && req.GetAutoscaling().GetEnabled() != tt.wantEnabled {
				t.Errorf("wanted enabled to be %v, but was %v", tt.wantEnabled, req.GetAutoscaling().GetEnabled())
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

type fakeResolver struct{}

func (f *fakeResolver) Resolve(_ context.Context, _, _ string) (*location.ResolvedLocation, error) {
	return &location.ResolvedLocation{
		Region:   "us-central1",
		Zone:     new("us-central1-a"),
		Regional: false,
	}, nil
}

// newReconcileTestService returns a Service backed by a fake GKE API that always returns nodePool.
// The machine pool starts as ready, so tests can check that a reconcile marks it as not ready.
func newReconcileTestService(t *testing.T, nodePool *containerpb.NodePool, replicas *int32) *Service {
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

	machinePool := defaultMachinePool()
	machinePool.Spec.Replicas = replicas

	s, err := scope.NewManagedMachinePoolScope(ctx, scope.ManagedMachinePoolScopeParams{
		Client:                      fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		Cluster:                     &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}},
		MachinePool:                 machinePool,
		GCPManagedCluster:           &infrav1exp.GCPManagedCluster{},
		GCPManagedControlPlane:      defaultControlPlane(),
		GCPManagedMachinePool:       gcpMachinePool,
		ManagedClusterClient:        mcClient,
		InstanceGroupManagersClient: migClient,
		InstanceTemplatesClient:     templatesClient,
		Resolver:                    &fakeResolver{},
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
		replicas    *int32
		wantErr     bool
		wantReason  string
		wantMessage string
		wantRequeue bool
	}{
		{
			name: "node pool in error",
			nodePool: &containerpb.NodePool{
				Status:     containerpb.NodePool_ERROR,
				Conditions: []*containerpb.StatusCondition{{Message: "quota exceeded"}},
			},
			replicas:    new(int32(3)),
			wantReason:  infrav1exp.GKEMachinePoolErrorReason,
			wantMessage: "quota exceeded",
			wantRequeue: false,
		},
		{
			name:        "node pool running with error",
			nodePool:    &containerpb.NodePool{Status: containerpb.NodePool_RUNNING_WITH_ERROR},
			replicas:    new(int32(3)),
			wantReason:  infrav1exp.GKEMachinePoolErrorReason,
			wantRequeue: false,
		},
		{
			name:        "node pool provisioning",
			nodePool:    &containerpb.NodePool{Status: containerpb.NodePool_PROVISIONING},
			replicas:    new(int32(3)),
			wantReason:  infrav1exp.GKEMachinePoolCreatingReason,
			wantRequeue: true,
		},
		{
			name:        "node pool stopping",
			nodePool:    &containerpb.NodePool{Status: containerpb.NodePool_STOPPING},
			replicas:    new(int32(3)),
			wantReason:  infrav1exp.GKEMachinePoolDeletingReason,
			wantRequeue: false,
		},
		{
			name: "node pool running with non-multiple replicas",
			nodePool: &containerpb.NodePool{Status: containerpb.NodePool_RUNNING, Locations: []string{
				"us-central1-a", "us-central1-b", "us-central-1c",
			}},
			replicas:    new(int32(4)),
			wantReason:  infrav1exp.GKEMachinePoolReconciliationFailedReason,
			wantMessage: "updating node pool size: invalid replicas: 4 is not a multiple of the zone count (3)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			svc := newReconcileTestService(t, tt.nodePool, tt.replicas)
			res, err := svc.Reconcile(ctx)
			if (res != ctrl.Result{}) != tt.wantRequeue {
				t.Errorf("Reconcile() got = %v, wanted requeue to be %v", res, tt.wantRequeue)
				return
			}
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
