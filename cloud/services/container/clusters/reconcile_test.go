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
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/protobuf/types/known/durationpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func newTestService(controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedControlPlaneScope)
	s.GCPManagedControlPlane = controlPlane
	return &Service{scope: s}
}

func TestCheckDiffAndPrepareUpdate(t *testing.T) {
	tests := []struct {
		name               string
		controlPlane       *infrav1exp.GCPManagedControlPlane
		existingCluster    *containerpb.Cluster
		wantNeedUpdate     bool
		wantUpdateNotNil   bool
		validateUpdateFunc func(t *testing.T, req *containerpb.UpdateClusterRequest)
	}{
		{
			name: "no diff when everything matches",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:        "test-project",
						Location:       "us-central1",
						ReleaseChannel: ptr.To(infrav1exp.Stable),
						ClusterName:    "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				ReleaseChannel: &containerpb.ReleaseChannel{
					Channel: containerpb.ReleaseChannel_STABLE,
				},
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{
						AuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
							Enabled:                     false,
							CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
							GcpPublicCidrsAccessEnabled: ptr.To(false),
						},
					},
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "update needed when release channel differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:        "test-project",
						Location:       "us-central1",
						ReleaseChannel: ptr.To(infrav1exp.Rapid),
						ClusterName:    "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				ReleaseChannel: &containerpb.ReleaseChannel{
					Channel: containerpb.ReleaseChannel_STABLE,
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredReleaseChannel().GetChannel() != containerpb.ReleaseChannel_RAPID {
					t.Errorf("expected RAPID release channel, got %v", req.GetUpdate().GetDesiredReleaseChannel().GetChannel())
				}
			},
		},
		{
			name: "update needed when master version differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
					},
					Version: ptr.To("1.28.0"),
				},
			},
			existingCluster: &containerpb.Cluster{
				CurrentMasterVersion: "1.27.2-gke.2100",
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredMasterVersion() != "1.28.0" {
					t.Errorf("expected master version 1.28.0, got %v", req.GetUpdate().GetDesiredMasterVersion())
				}
			},
		},
		{
			name: "update needed when monitoring service differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						ClusterName:       "test-cluster",
						Project:           "test-project",
						Location:          "us-central1",
						MonitoringService: ptr.To(infrav1exp.MonitoringService("none")),
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				MonitoringService: "monitoring.googleapis.com/kubernetes",
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredMonitoringService() != "none" {
					t.Errorf("expected DesiredMonitoringService to be set to %q, got %q", "none", req.GetUpdate().GetDesiredMonitoringService())
				}
				if req.GetUpdate().GetDesiredLoggingService() != "" {
					t.Errorf("expected DesiredLoggingService to be left unset, got %q", req.GetUpdate().GetDesiredLoggingService())
				}
			},
		},
		{
			name: "no panic when existing cluster has nil ControlPlaneEndpointsConfig",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{},
			// needUpdate is true because nil spec MasterAuthorizedNetworksConfig generates
			// a "disabled" config which differs from the nil existing config.
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig() == nil {
					t.Fatal("expected DesiredControlPlaneEndpointsConfig to be initialized")
				}
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig().GetIpEndpointsConfig() == nil {
					t.Fatal("expected IpEndpointsConfig to be initialized")
				}
			},
		},
		{
			name: "no panic when existing cluster has nil IpEndpointsConfig",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig() == nil {
					t.Fatal("expected DesiredControlPlaneEndpointsConfig to be initialized")
				}
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig().GetIpEndpointsConfig() == nil {
					t.Fatal("expected IpEndpointsConfig to be initialized")
				}
			},
		},
		{
			name: "authorized networks update initializes parent structs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:  "test-project",
						Location: "us-central1",
						MasterAuthorizedNetworksConfig: &infrav1exp.MasterAuthorizedNetworksConfig{
							CidrBlocks: []*infrav1exp.MasterAuthorizedNetworksConfigCidrBlock{
								{CidrBlock: "10.0.0.0/8", DisplayName: "internal"},
							},
						},
						ClusterName: "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{},
			wantNeedUpdate:  true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig() == nil {
					t.Fatal("expected DesiredControlPlaneEndpointsConfig to be initialized")
				}
				if req.GetUpdate().GetDesiredControlPlaneEndpointsConfig().GetIpEndpointsConfig() == nil {
					t.Fatal("expected IpEndpointsConfig to be initialized")
				}
				authConfig := req.GetUpdate().GetDesiredControlPlaneEndpointsConfig().GetIpEndpointsConfig().GetAuthorizedNetworksConfig()
				if authConfig == nil {
					t.Fatal("expected AuthorizedNetworksConfig to be set")
				}
				if !authConfig.GetEnabled() {
					t.Error("expected AuthorizedNetworksConfig to be enabled")
				}
				if len(authConfig.GetCidrBlocks()) != 1 || authConfig.GetCidrBlocks()[0].GetCidrBlock() != "10.0.0.0/8" {
					t.Errorf("unexpected CidrBlocks: %v", authConfig.GetCidrBlocks())
				}
			},
		},
		{
			name: "no diff when gateway api channel matches",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						ClusterName: "test-cluster",
						Project:     "test-project",
						Location:    "us-central1",
						ClusterNetwork: &infrav1exp.ClusterNetwork{
							GatewayAPIChannel: ptr.To(infrav1exp.GatewayAPIChannelStandard),
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				NetworkConfig: &containerpb.NetworkConfig{
					GatewayApiConfig: &containerpb.GatewayAPIConfig{
						Channel: containerpb.GatewayAPIConfig_CHANNEL_STANDARD,
					},
				},
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{
						AuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
							Enabled:                     false,
							CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
							GcpPublicCidrsAccessEnabled: ptr.To(false),
						},
					},
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "update needed when gateway api channel differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						ClusterName: "test-cluster",
						Project:     "test-project",
						Location:    "us-central1",
						ClusterNetwork: &infrav1exp.ClusterNetwork{
							GatewayAPIChannel: ptr.To(infrav1exp.GatewayAPIChannelStandard),
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				NetworkConfig: &containerpb.NetworkConfig{
					GatewayApiConfig: &containerpb.GatewayAPIConfig{
						Channel: containerpb.GatewayAPIConfig_CHANNEL_DISABLED,
					},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredGatewayApiConfig().GetChannel() != containerpb.GatewayAPIConfig_CHANNEL_STANDARD {
					t.Errorf("expected STANDARD gateway API channel, got %v", req.GetUpdate().GetDesiredGatewayApiConfig().GetChannel())
				}
			},
		},
		{
			name: "update needed when gateway api channel desired but existing cluster has no NetworkConfig",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						ClusterName: "test-cluster",
						Project:     "test-project",
						Location:    "us-central1",
						ClusterNetwork: &infrav1exp.ClusterNetwork{
							GatewayAPIChannel: ptr.To(infrav1exp.GatewayAPIChannelDisabled),
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{},
			wantNeedUpdate:  true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if req.GetUpdate().GetDesiredGatewayApiConfig().GetChannel() != containerpb.GatewayAPIConfig_CHANNEL_DISABLED {
					t.Errorf("expected DISABLED gateway API channel, got %v", req.GetUpdate().GetDesiredGatewayApiConfig().GetChannel())
				}
			},
		},
		{
			name: "authorized networks update with existing cluster having nil nested config",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:  "test-project",
						Location: "us-central1",
						MasterAuthorizedNetworksConfig: &infrav1exp.MasterAuthorizedNetworksConfig{
							CidrBlocks: []*infrav1exp.MasterAuthorizedNetworksConfigCidrBlock{
								{CidrBlock: "192.168.0.0/16"},
							},
						},
						ClusterName: "test-cluster",
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{
						AuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
							Enabled: true,
							CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
								{CidrBlock: "10.0.0.0/8"},
							},
						},
					},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				authConfig := req.GetUpdate().GetDesiredControlPlaneEndpointsConfig().GetIpEndpointsConfig().GetAuthorizedNetworksConfig()
				if len(authConfig.GetCidrBlocks()) != 1 || authConfig.GetCidrBlocks()[0].GetCidrBlock() != "192.168.0.0/16" {
					t.Errorf("unexpected CidrBlocks: %v", authConfig.GetCidrBlocks())
				}
			},
		},
		{
			name: "no update needed when secret manager config already matches",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretManagerConfig: &infrav1exp.SecretManagerConfig{
							Enabled: true,
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				SecretManagerConfig: &containerpb.SecretManagerConfig{
					Enabled: ptr.To(true),
				},
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{
						AuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
							Enabled:                     false,
							CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
							GcpPublicCidrsAccessEnabled: ptr.To(false),
						},
					},
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "update needed when secret manager config enabled differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretManagerConfig: &infrav1exp.SecretManagerConfig{
							Enabled: true,
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{},
			wantNeedUpdate:  true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if !req.GetUpdate().GetDesiredSecretManagerConfig().GetEnabled() {
					t.Errorf("expected Secret Manager config to be enabled")
				}
			},
		},
		{
			name: "update needed when secret manager rotation interval differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretManagerConfig: &infrav1exp.SecretManagerConfig{
							Enabled: true,
							RotationConfig: &infrav1exp.SecretManagerRotationConfig{
								Enabled:          true,
								RotationInterval: &metav1.Duration{Duration: 5 * time.Minute},
							},
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				SecretManagerConfig: &containerpb.SecretManagerConfig{
					Enabled: ptr.To(true),
					RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
						Enabled:          ptr.To(true),
						RotationInterval: durationpb.New(2 * time.Minute),
					},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				gotInterval := req.GetUpdate().GetDesiredSecretManagerConfig().GetRotationConfig().GetRotationInterval().AsDuration()
				if gotInterval != 5*time.Minute {
					t.Errorf("expected rotation interval of 5m, got %v", gotInterval)
				}
			},
		},
		{
			name: "no update needed when secret sync config already matches",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretSyncConfig: &infrav1exp.SecretSyncConfig{
							Enabled: true,
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				SecretSyncConfig: &containerpb.SecretSyncConfig{
					Enabled: ptr.To(true),
				},
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{
						AuthorizedNetworksConfig: &containerpb.MasterAuthorizedNetworksConfig{
							Enabled:                     false,
							CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
							GcpPublicCidrsAccessEnabled: ptr.To(false),
						},
					},
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "update needed when secret sync config enabled differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretSyncConfig: &infrav1exp.SecretSyncConfig{
							Enabled: true,
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{},
			wantNeedUpdate:  true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				if !req.GetUpdate().GetDesiredSecretSyncConfig().GetEnabled() {
					t.Errorf("expected Secret Sync config to be enabled")
				}
			},
		},
		{
			name: "update needed when secret sync rotation interval differs",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:     "test-project",
						Location:    "us-central1",
						ClusterName: "test-cluster",
						SecretSyncConfig: &infrav1exp.SecretSyncConfig{
							Enabled: true,
							RotationConfig: &infrav1exp.SecretSyncRotationConfig{
								Enabled:          true,
								RotationInterval: &metav1.Duration{Duration: 5 * time.Minute},
							},
						},
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				SecretSyncConfig: &containerpb.SecretSyncConfig{
					Enabled: ptr.To(true),
					RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
						Enabled:          ptr.To(true),
						RotationInterval: durationpb.New(2 * time.Minute),
					},
				},
			},
			wantNeedUpdate: true,
			validateUpdateFunc: func(t *testing.T, req *containerpb.UpdateClusterRequest) {
				t.Helper()
				gotInterval := req.GetUpdate().GetDesiredSecretSyncConfig().GetRotationConfig().GetRotationInterval().AsDuration()
				if gotInterval != 5*time.Minute {
					t.Errorf("expected rotation interval of 5m, got %v", gotInterval)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(tt.controlPlane)
			log := ctrl.Log.WithName("test")
			needUpdate, updateReq := svc.checkDiffAndPrepareUpdate(tt.existingCluster, &log)
			if needUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdate() needUpdate = %v, want %v", needUpdate, tt.wantNeedUpdate)
			}
			if tt.validateUpdateFunc != nil {
				tt.validateUpdateFunc(t, updateReq)
			}
		})
	}
}

func TestCompareMasterAuthorizedNetworksConfig(t *testing.T) {
	tests := []struct {
		name string
		a    *containerpb.MasterAuthorizedNetworksConfig
		b    *containerpb.MasterAuthorizedNetworksConfig
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "a nil b not nil",
			a:    nil,
			b:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: true},
			want: false,
		},
		{
			name: "a not nil b nil",
			a:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: true},
			b:    nil,
			want: false,
		},
		{
			name: "both equal enabled",
			a:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: true},
			b:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: true},
			want: true,
		},
		{
			name: "different enabled",
			a:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: true},
			b:    &containerpb.MasterAuthorizedNetworksConfig{Enabled: false},
			want: false,
		},
		{
			name: "same cidr blocks",
			a: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled: true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
					{CidrBlock: "10.0.0.0/8", DisplayName: "test"},
				},
			},
			b: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled: true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
					{CidrBlock: "10.0.0.0/8", DisplayName: "test"},
				},
			},
			want: true,
		},
		{
			name: "different cidr blocks",
			a: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled: true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
					{CidrBlock: "10.0.0.0/8"},
				},
			},
			b: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled: true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
					{CidrBlock: "192.168.0.0/16"},
				},
			},
			want: false,
		},
		{
			name: "nil vs empty cidr blocks are equal",
			a: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:    true,
				CidrBlocks: nil,
			},
			b: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:    true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
			},
			want: true,
		},
		{
			name: "different GcpPublicCidrsAccessEnabled",
			a: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:                     true,
				GcpPublicCidrsAccessEnabled: ptr.To(true),
			},
			b: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:                     true,
				GcpPublicCidrsAccessEnabled: ptr.To(false),
			},
			want: false,
		},
		{
			name: "one GcpPublicCidrsAccessEnabled nil other set",
			a: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:                     true,
				GcpPublicCidrsAccessEnabled: nil,
			},
			b: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:                     true,
				GcpPublicCidrsAccessEnabled: ptr.To(true),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareMasterAuthorizedNetworksConfig(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareMasterAuthorizedNetworksConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkMasterAuthorizedNetworksConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *infrav1exp.MasterAuthorizedNetworksConfig
		want   *containerpb.MasterAuthorizedNetworksConfig
	}{
		{
			name:   "nil config returns disabled",
			config: nil,
			want: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:                     false,
				CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
				GcpPublicCidrsAccessEnabled: new(bool),
			},
		},
		{
			name: "config with cidr blocks",
			config: &infrav1exp.MasterAuthorizedNetworksConfig{
				CidrBlocks: []*infrav1exp.MasterAuthorizedNetworksConfigCidrBlock{
					{CidrBlock: "10.0.0.0/8", DisplayName: "internal"},
				},
				GcpPublicCidrsAccessEnabled: ptr.To(true),
			},
			want: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled: true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
					{CidrBlock: "10.0.0.0/8", DisplayName: "internal"},
				},
				GcpPublicCidrsAccessEnabled: ptr.To(true),
			},
		},
		{
			name:   "empty config",
			config: &infrav1exp.MasterAuthorizedNetworksConfig{},
			want: &containerpb.MasterAuthorizedNetworksConfig{
				Enabled:    true,
				CidrBlocks: []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkMasterAuthorizedNetworksConfig(tt.config)
			if !compareMasterAuthorizedNetworksConfig(got, tt.want) {
				t.Errorf("convertToSdkMasterAuthorizedNetworksConfig() = %v, want %v", got, tt.want)
			}
			if got.GetEnabled() != tt.want.GetEnabled() {
				t.Errorf("Enabled = %v, want %v", got.GetEnabled(), tt.want.GetEnabled())
			}
		})
	}
}

func TestCompareSecretManagerConfig(t *testing.T) {
	tests := []struct {
		name string
		a    *containerpb.SecretManagerConfig
		b    *containerpb.SecretManagerConfig
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "both disabled",
			a:    &containerpb.SecretManagerConfig{Enabled: ptr.To(false)},
			b:    &containerpb.SecretManagerConfig{Enabled: ptr.To(false)},
			want: true,
		},
		{
			name: "different enabled",
			a:    &containerpb.SecretManagerConfig{Enabled: ptr.To(true)},
			b:    &containerpb.SecretManagerConfig{Enabled: ptr.To(false)},
			want: false,
		},
		{
			name: "same rotation config",
			a: &containerpb.SecretManagerConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			b: &containerpb.SecretManagerConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			want: true,
		},
		{
			name: "different rotation interval",
			a: &containerpb.SecretManagerConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			b: &containerpb.SecretManagerConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(5 * time.Minute),
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSecretManagerConfig(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareSecretManagerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkSecretManagerConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *infrav1exp.SecretManagerConfig
		want   *containerpb.SecretManagerConfig
	}{
		{
			name:   "nil config returns disabled",
			config: nil,
			want:   &containerpb.SecretManagerConfig{Enabled: ptr.To(false)},
		},
		{
			name: "enabled with no rotation config",
			config: &infrav1exp.SecretManagerConfig{
				Enabled: true,
			},
			want: &containerpb.SecretManagerConfig{Enabled: ptr.To(true)},
		},
		{
			name: "enabled with rotation config",
			config: &infrav1exp.SecretManagerConfig{
				Enabled: true,
				RotationConfig: &infrav1exp.SecretManagerRotationConfig{
					Enabled:          true,
					RotationInterval: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
			want: &containerpb.SecretManagerConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretManagerConfig_RotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(5 * time.Minute),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkSecretManagerConfig(tt.config)
			if !compareSecretManagerConfig(got, tt.want) {
				t.Errorf("convertToSdkSecretManagerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareSecretSyncConfig(t *testing.T) {
	tests := []struct {
		name string
		a    *containerpb.SecretSyncConfig
		b    *containerpb.SecretSyncConfig
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "both disabled",
			a:    &containerpb.SecretSyncConfig{Enabled: ptr.To(false)},
			b:    &containerpb.SecretSyncConfig{Enabled: ptr.To(false)},
			want: true,
		},
		{
			name: "different enabled",
			a:    &containerpb.SecretSyncConfig{Enabled: ptr.To(true)},
			b:    &containerpb.SecretSyncConfig{Enabled: ptr.To(false)},
			want: false,
		},
		{
			name: "same rotation config",
			a: &containerpb.SecretSyncConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			b: &containerpb.SecretSyncConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			want: true,
		},
		{
			name: "different rotation interval",
			a: &containerpb.SecretSyncConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(2 * time.Minute),
				},
			},
			b: &containerpb.SecretSyncConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(5 * time.Minute),
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSecretSyncConfig(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareSecretSyncConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkSecretSyncConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *infrav1exp.SecretSyncConfig
		want   *containerpb.SecretSyncConfig
	}{
		{
			name:   "nil config returns disabled",
			config: nil,
			want:   &containerpb.SecretSyncConfig{Enabled: ptr.To(false)},
		},
		{
			name: "enabled with no rotation config",
			config: &infrav1exp.SecretSyncConfig{
				Enabled: true,
			},
			want: &containerpb.SecretSyncConfig{Enabled: ptr.To(true)},
		},
		{
			name: "enabled with rotation config",
			config: &infrav1exp.SecretSyncConfig{
				Enabled: true,
				RotationConfig: &infrav1exp.SecretSyncRotationConfig{
					Enabled:          true,
					RotationInterval: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
			want: &containerpb.SecretSyncConfig{
				Enabled: ptr.To(true),
				RotationConfig: &containerpb.SecretSyncConfig_SyncRotationConfig{
					Enabled:          ptr.To(true),
					RotationInterval: durationpb.New(5 * time.Minute),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkSecretSyncConfig(tt.config)
			if !compareSecretSyncConfig(got, tt.want) {
				t.Errorf("convertToSdkSecretSyncConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkReleaseChannel(t *testing.T) {
	tests := []struct {
		name    string
		channel *infrav1exp.ReleaseChannel
		want    containerpb.ReleaseChannel_Channel
	}{
		{
			name:    "nil channel",
			channel: nil,
			want:    containerpb.ReleaseChannel_UNSPECIFIED,
		},
		{
			name:    "rapid",
			channel: ptr.To(infrav1exp.Rapid),
			want:    containerpb.ReleaseChannel_RAPID,
		},
		{
			name:    "regular",
			channel: ptr.To(infrav1exp.Regular),
			want:    containerpb.ReleaseChannel_REGULAR,
		},
		{
			name:    "stable",
			channel: ptr.To(infrav1exp.Stable),
			want:    containerpb.ReleaseChannel_STABLE,
		},
		{
			name:    "extended",
			channel: ptr.To(infrav1exp.Extended),
			want:    containerpb.ReleaseChannel_EXTENDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkReleaseChannel(tt.channel)
			if got != tt.want {
				t.Errorf("convertToSdkReleaseChannel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkMasterVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "version with gke suffix",
			version: "1.27.2-gke.2100",
			want:    "1.27.2",
		},
		{
			name:    "version without suffix",
			version: "1.27.2",
			want:    "1.27.2",
		},
		{
			name:    "version with v prefix",
			version: "v1.27.2-gke.2100",
			want:    "1.27.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkMasterVersion(tt.version)
			if got != tt.want {
				t.Errorf("convertToSdkMasterVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToSdkBinaryAuthorizationEvaluationMode(t *testing.T) {
	tests := []struct {
		name string
		mode *infrav1exp.BinaryAuthorization
		want containerpb.BinaryAuthorization_EvaluationMode
	}{
		{
			name: "nil mode",
			mode: nil,
			want: containerpb.BinaryAuthorization_EVALUATION_MODE_UNSPECIFIED,
		},
		{
			name: "disabled",
			mode: ptr.To(infrav1exp.EvaluationModeDisabled),
			want: containerpb.BinaryAuthorization_DISABLED,
		},
		{
			name: "project singleton policy enforce",
			mode: ptr.To(infrav1exp.EvaluationModeProjectSingletonPolicyEnforce),
			want: containerpb.BinaryAuthorization_PROJECT_SINGLETON_POLICY_ENFORCE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkBinaryAuthorizationEvaluationMode(tt.mode)
			if got != tt.want {
				t.Errorf("convertToSdkBinaryAuthorizationEvaluationMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterNetworkNilPointerGuards(t *testing.T) {
	tests := []struct {
		name           string
		clusterNetwork *infrav1exp.ClusterNetwork
	}{
		{
			name: "no panic with gateway api channel only and no private cluster",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				GatewayAPIChannel: ptr.To(infrav1exp.GatewayAPIChannelStandard),
			},
		},
		{
			name: "no panic with gateway api channel and private cluster combined",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				GatewayAPIChannel: ptr.To(infrav1exp.GatewayAPIChannelStandard),
				PrivateCluster: &infrav1exp.PrivateCluster{
					EnablePrivateNodes:    true,
					EnablePrivateEndpoint: true,
					ControlPlaneCidrBlock: "172.16.0.0/28",
				},
			},
		},
		{
			name: "no panic when useIPAliases is true with nil Pod and nil Service",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				UseIPAliases: true,
				Pod:          nil,
				Service:      nil,
			},
		},
		{
			name: "no panic when useIPAliases is true with Pod set but nil Service",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				UseIPAliases: true,
				Pod:          &infrav1exp.ClusterNetworkPod{CidrBlock: "10.88.0.0/16"},
				Service:      nil,
			},
		},
		{
			name: "no panic when useIPAliases is true with nil Pod but Service set",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				UseIPAliases: true,
				Pod:          nil,
				Service:      &infrav1exp.ClusterNetworkService{CidrBlock: "10.89.0.0/16"},
			},
		},
		{
			name: "no panic with private cluster and useIPAliases combined",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				UseIPAliases: true,
				Pod:          &infrav1exp.ClusterNetworkPod{CidrBlock: "10.88.0.0/16"},
				PrivateCluster: &infrav1exp.PrivateCluster{
					EnablePrivateNodes:    true,
					EnablePrivateEndpoint: true,
					ControlPlaneCidrBlock: "172.16.0.0/28",
				},
			},
		},
		{
			name: "no panic with private cluster only and no useIPAliases",
			clusterNetwork: &infrav1exp.ClusterNetwork{
				PrivateCluster: &infrav1exp.PrivateCluster{
					EnablePrivateNodes:    true,
					EnablePrivateEndpoint: true,
					ControlPlaneCidrBlock: "172.16.0.0/28",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a cluster proto the same way createCluster does, to verify
			// no nil pointer dereference occurs with various ClusterNetwork configs.
			cluster := &containerpb.Cluster{
				ControlPlaneEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig{
					IpEndpointsConfig: &containerpb.ControlPlaneEndpointsConfig_IPEndpointsConfig{},
				},
			}
			cn := tt.clusterNetwork

			if cn.UseIPAliases {
				cluster.IpAllocationPolicy = &containerpb.IPAllocationPolicy{}
				cluster.IpAllocationPolicy.UseIpAliases = cn.UseIPAliases
				if cn.Pod != nil {
					cluster.IpAllocationPolicy.ClusterIpv4CidrBlock = cn.Pod.CidrBlock
				}
				if cn.Service != nil {
					cluster.IpAllocationPolicy.ServicesIpv4CidrBlock = cn.Service.CidrBlock
				}
			}

			if cn.PrivateCluster != nil {
				enablePublicEndpoint := !cn.PrivateCluster.EnablePrivateEndpoint
				cluster.ControlPlaneEndpointsConfig.IpEndpointsConfig.EnablePublicEndpoint = &enablePublicEndpoint
				if cn.PrivateCluster.EnablePrivateEndpoint {
					cluster.ControlPlaneEndpointsConfig.IpEndpointsConfig.AuthorizedNetworksConfig = &containerpb.MasterAuthorizedNetworksConfig{
						Enabled: true,
					}
				}

				if cluster.GetNetworkConfig() == nil {
					cluster.NetworkConfig = &containerpb.NetworkConfig{}
				}
				cluster.NetworkConfig.DefaultSnatStatus = &containerpb.DefaultSnatStatus{
					Disabled: cn.PrivateCluster.DisableDefaultSNAT,
				}
				cluster.NetworkConfig.DefaultEnablePrivateNodes = &cn.PrivateCluster.EnablePrivateNodes

				cluster.PrivateClusterConfig = &containerpb.PrivateClusterConfig{
					MasterIpv4CidrBlock: cn.PrivateCluster.ControlPlaneCidrBlock,
					EnablePrivateNodes:  cn.PrivateCluster.EnablePrivateNodes,
				}
				cluster.ControlPlaneEndpointsConfig.IpEndpointsConfig.GlobalAccess = &cn.PrivateCluster.ControlPlaneGlobalAccess
			}

			if cn.GatewayAPIChannel != nil {
				if cluster.GetNetworkConfig() == nil {
					cluster.NetworkConfig = &containerpb.NetworkConfig{}
				}
				cluster.NetworkConfig.GatewayApiConfig = &containerpb.GatewayAPIConfig{
					Channel: convertToSdkGatewayAPIChannel(cn.GatewayAPIChannel),
				}
			}

			// Verify IP allocation policy when UseIPAliases is set
			if cn.UseIPAliases {
				if cluster.GetIpAllocationPolicy() == nil {
					t.Fatal("expected IpAllocationPolicy to be set")
				}
				if !cluster.GetIpAllocationPolicy().GetUseIpAliases() {
					t.Error("expected UseIpAliases to be true")
				}
				if cn.Pod != nil && cluster.GetIpAllocationPolicy().GetClusterIpv4CidrBlock() != cn.Pod.CidrBlock {
					t.Errorf("expected ClusterIpv4CidrBlock %q, got %q", cn.Pod.CidrBlock, cluster.GetIpAllocationPolicy().GetClusterIpv4CidrBlock())
				}
				if cn.Pod == nil && cluster.GetIpAllocationPolicy().GetClusterIpv4CidrBlock() != "" {
					t.Errorf("expected empty ClusterIpv4CidrBlock when Pod is nil, got %q", cluster.GetIpAllocationPolicy().GetClusterIpv4CidrBlock())
				}
				if cn.Service != nil && cluster.GetIpAllocationPolicy().GetServicesIpv4CidrBlock() != cn.Service.CidrBlock {
					t.Errorf("expected ServicesIpv4CidrBlock %q, got %q", cn.Service.CidrBlock, cluster.GetIpAllocationPolicy().GetServicesIpv4CidrBlock())
				}
				if cn.Service == nil && cluster.GetIpAllocationPolicy().GetServicesIpv4CidrBlock() != "" {
					t.Errorf("expected empty ServicesIpv4CidrBlock when Service is nil, got %q", cluster.GetIpAllocationPolicy().GetServicesIpv4CidrBlock())
				}
			}

			// Verify private cluster config
			if cn.PrivateCluster != nil {
				if cluster.GetNetworkConfig() == nil {
					t.Fatal("expected NetworkConfig to be initialized")
				}
				if cluster.GetNetworkConfig().GetDefaultEnablePrivateNodes() != cn.PrivateCluster.EnablePrivateNodes {
					t.Errorf("expected DefaultEnablePrivateNodes %v, got %v", cn.PrivateCluster.EnablePrivateNodes, cluster.GetNetworkConfig().GetDefaultEnablePrivateNodes())
				}
				if cluster.GetPrivateClusterConfig() == nil {
					t.Fatal("expected PrivateClusterConfig to be set")
				}
			}

			// Verify gateway API config
			if cn.GatewayAPIChannel != nil {
				if cluster.GetNetworkConfig() == nil {
					t.Fatal("expected NetworkConfig to be initialized")
				}
				if cluster.GetNetworkConfig().GetGatewayApiConfig().GetChannel() != containerpb.GatewayAPIConfig_CHANNEL_STANDARD {
					t.Errorf("expected STANDARD gateway API channel, got %v", cluster.GetNetworkConfig().GetGatewayApiConfig().GetChannel())
				}
			}
		})
	}
}
