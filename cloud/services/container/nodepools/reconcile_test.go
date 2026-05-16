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
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

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

func TestCheckDiffAndPrepareUpdateConfig_WorkloadMetadataMode(t *testing.T) {
	controlPlane := &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
				Project:  "test-project",
				Location: "us-central1-a",
			},
			ClusterName: "test-cluster",
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
			_, updateReq := svc.checkDiffAndPrepareUpdateConfig(tt.existingNodePool)

			if tt.wantMetadataMode == nil {
				if updateReq.WorkloadMetadataConfig != nil {
					t.Errorf("expected nil WorkloadMetadataConfig, got %v", updateReq.WorkloadMetadataConfig)
				}
			} else if mode := updateReq.WorkloadMetadataConfig.GetMode(); mode != *tt.wantMetadataMode {
				t.Errorf("expected WorkloadMetadataConfig.Mode %v, got %v", *tt.wantMetadataMode, mode)
			}
		})
	}
}
