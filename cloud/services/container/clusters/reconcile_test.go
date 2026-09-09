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
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func newTestService(controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedControlPlaneScope)
	s.GCPManagedControlPlane = controlPlane
	return &Service{scope: s}
}

// TestNetworkName guards against a regression of
// https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues/1187: createCluster used to
// dereference Spec.Network.Name directly, which panicked whenever a GCPManagedCluster was created
// without the network field set. Network is a non-pointer struct field, so omitting it from a
// manifest leaves it as the zero-value NetworkSpec{} (Name == nil), which is what "network name
// not set" below represents.
func TestNetworkName(t *testing.T) {
	tests := []struct {
		name    string
		network infrav1.NetworkSpec
		want    string
	}{
		{
			name:    "network name not set",
			network: infrav1.NetworkSpec{},
			want:    "",
		},
		{
			name:    "network name set",
			network: infrav1.NetworkSpec{Name: ptr.To("my-network")},
			want:    "my-network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(&infrav1exp.GCPManagedControlPlane{})
			svc.scope.GCPManagedCluster = &infrav1exp.GCPManagedCluster{
				Spec: infrav1exp.GCPManagedClusterSpec{Network: tt.network},
			}
			if got := svc.networkName(); got != tt.want {
				t.Errorf("networkName() = %q, want %q", got, tt.want)
			}
		})
	}
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
			name: "no diff when gateway api channel is unset, regardless of what GKE actually has",
			controlPlane: &infrav1exp.GCPManagedControlPlane{
				Spec: infrav1exp.GCPManagedControlPlaneSpec{
					GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
						Project:        "test-project",
						Location:       "us-central1",
						ReleaseChannel: ptr.To(infrav1exp.Stable),
						ClusterName:    "test-cluster",
						// No ClusterNetwork/GatewayAPIChannel set - matches an
						// Autopilot cluster, which GKE mandates onto the
						// STANDARD channel regardless of what's requested.
					},
				},
			},
			existingCluster: &containerpb.Cluster{
				ReleaseChannel: &containerpb.ReleaseChannel{
					Channel: containerpb.ReleaseChannel_STABLE,
				},
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

func TestConvertToSdkMaintenancePolicy(t *testing.T) {
	start := metav1.NewTime(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	end := metav1.NewTime(time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC))

	tests := []struct {
		name   string
		policy *infrav1exp.MaintenancePolicy
		want   *containerpb.MaintenancePolicy
	}{
		{
			name:   "nil policy",
			policy: nil,
			want:   nil,
		},
		{
			name: "daily maintenance window",
			policy: &infrav1exp.MaintenancePolicy{
				DailyMaintenanceWindow: &infrav1exp.DailyMaintenanceWindow{StartTime: "03:00"},
			},
			want: &containerpb.MaintenancePolicy{
				Window: &containerpb.MaintenanceWindow{
					Policy: &containerpb.MaintenanceWindow_DailyMaintenanceWindow{
						DailyMaintenanceWindow: &containerpb.DailyMaintenanceWindow{StartTime: "03:00"},
					},
				},
			},
		},
		{
			name: "recurring maintenance window",
			policy: &infrav1exp.MaintenancePolicy{
				RecurringMaintenanceWindow: &infrav1exp.RecurringMaintenanceWindow{
					Window:     &infrav1exp.TimeWindow{StartTime: start, EndTime: end},
					Recurrence: "FREQ=WEEKLY;BYDAY=SA,SU",
				},
			},
			want: &containerpb.MaintenancePolicy{
				Window: &containerpb.MaintenanceWindow{
					Policy: &containerpb.MaintenanceWindow_RecurringWindow{
						RecurringWindow: &containerpb.RecurringTimeWindow{
							Window:     &containerpb.TimeWindow{StartTime: timestamppb.New(start.Time), EndTime: timestamppb.New(end.Time)},
							Recurrence: "FREQ=WEEKLY;BYDAY=SA,SU",
						},
					},
				},
			},
		},
		{
			// An unset MaintenanceExclusionOption is left as an unset Options field on the wire, rather than
			// an explicit NO_UPGRADES scope — GKE's own API default for an exclusion with no scope set is
			// already NO_UPGRADES, matching the "nil counts as no-upgrades" assumption the validating webhook
			// uses when counting exclusions (see validateMaintenancePolicy).
			name: "maintenance exclusion without an explicit option leaves scope unset",
			policy: &infrav1exp.MaintenancePolicy{
				MaintenanceExclusions: map[string]*infrav1exp.TimeWindow{
					"exclusion-1": {StartTime: start, EndTime: end},
				},
			},
			want: &containerpb.MaintenancePolicy{
				Window: &containerpb.MaintenanceWindow{
					MaintenanceExclusions: map[string]*containerpb.TimeWindow{
						"exclusion-1": {
							StartTime: timestamppb.New(start.Time),
							EndTime:   timestamppb.New(end.Time),
						},
					},
				},
			},
		},
		{
			name: "disruption budget",
			policy: &infrav1exp.MaintenancePolicy{
				DisruptionBudget: &infrav1exp.DisruptionBudget{
					MinorVersionDisruptionInterval: &metav1.Duration{Duration: 7 * 24 * time.Hour},
					PatchVersionDisruptionInterval: &metav1.Duration{Duration: 24 * time.Hour},
				},
			},
			want: &containerpb.MaintenancePolicy{
				DisruptionBudget: &containerpb.DisruptionBudget{
					MinorVersionDisruptionInterval: durationpb.New(7 * 24 * time.Hour),
					PatchVersionDisruptionInterval: durationpb.New(24 * time.Hour),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToSdkMaintenancePolicy(tt.policy)
			if !proto.Equal(got, tt.want) {
				t.Errorf("convertToSdkMaintenancePolicy() mismatch (-got +want):\n%s", cmp.Diff(got, tt.want, protocmp.Transform()))
			}
		})
	}
}

func TestCheckDiffAndPrepareUpdateMaintenancePolicy(t *testing.T) {
	start := metav1.NewTime(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
	end := metav1.NewTime(time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC))

	baseControlPlane := func(policy *infrav1exp.MaintenancePolicy) *infrav1exp.GCPManagedControlPlane {
		return &infrav1exp.GCPManagedControlPlane{
			Spec: infrav1exp.GCPManagedControlPlaneSpec{
				GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
					Project:           "test-project",
					Location:          "us-central1",
					ClusterName:       "test-cluster",
					MaintenancePolicy: policy,
				},
			},
		}
	}

	tests := []struct {
		name            string
		controlPlane    *infrav1exp.GCPManagedControlPlane
		existingCluster *containerpb.Cluster
		wantNeedUpdate  bool
	}{
		{
			name:            "no policy desired, none exists",
			controlPlane:    baseControlPlane(nil),
			existingCluster: &containerpb.Cluster{},
			wantNeedUpdate:  false,
		},
		{
			name: "no diff when daily window matches",
			controlPlane: baseControlPlane(&infrav1exp.MaintenancePolicy{
				DailyMaintenanceWindow: &infrav1exp.DailyMaintenanceWindow{StartTime: "03:00"},
			}),
			existingCluster: &containerpb.Cluster{
				MaintenancePolicy: &containerpb.MaintenancePolicy{
					ResourceVersion: "abc123",
					Window: &containerpb.MaintenanceWindow{
						Policy: &containerpb.MaintenanceWindow_DailyMaintenanceWindow{
							DailyMaintenanceWindow: &containerpb.DailyMaintenanceWindow{StartTime: "03:00"},
						},
					},
				},
			},
			wantNeedUpdate: false,
		},
		{
			name: "update needed when daily window differs",
			controlPlane: baseControlPlane(&infrav1exp.MaintenancePolicy{
				DailyMaintenanceWindow: &infrav1exp.DailyMaintenanceWindow{StartTime: "05:00"},
			}),
			existingCluster: &containerpb.Cluster{
				MaintenancePolicy: &containerpb.MaintenancePolicy{
					ResourceVersion: "abc123",
					Window: &containerpb.MaintenanceWindow{
						Policy: &containerpb.MaintenanceWindow_DailyMaintenanceWindow{
							DailyMaintenanceWindow: &containerpb.DailyMaintenanceWindow{StartTime: "03:00"},
						},
					},
				},
			},
			wantNeedUpdate: true,
		},
		{
			name:         "update needed to remove an existing policy",
			controlPlane: baseControlPlane(nil),
			existingCluster: &containerpb.Cluster{
				MaintenancePolicy: &containerpb.MaintenancePolicy{
					ResourceVersion: "abc123",
					Window: &containerpb.MaintenanceWindow{
						Policy: &containerpb.MaintenanceWindow_DailyMaintenanceWindow{
							DailyMaintenanceWindow: &containerpb.DailyMaintenanceWindow{StartTime: "03:00"},
						},
					},
				},
			},
			wantNeedUpdate: true,
		},
		{
			name: "unrelated disruption budget observability fields don't trigger a diff",
			controlPlane: baseControlPlane(&infrav1exp.MaintenancePolicy{
				RecurringMaintenanceWindow: &infrav1exp.RecurringMaintenanceWindow{
					Window:     &infrav1exp.TimeWindow{StartTime: start, EndTime: end},
					Recurrence: "FREQ=WEEKLY;BYDAY=SA,SU",
				},
			}),
			existingCluster: &containerpb.Cluster{
				MaintenancePolicy: &containerpb.MaintenancePolicy{
					ResourceVersion: "abc123",
					Window: &containerpb.MaintenanceWindow{
						Policy: &containerpb.MaintenanceWindow_RecurringWindow{
							RecurringWindow: &containerpb.RecurringTimeWindow{
								Window:     &containerpb.TimeWindow{StartTime: timestamppb.New(start.Time), EndTime: timestamppb.New(end.Time)},
								Recurrence: "FREQ=WEEKLY;BYDAY=SA,SU",
							},
						},
					},
					DisruptionBudget: &containerpb.DisruptionBudget{
						LastDisruptionTime: timestamppb.New(start.Time),
					},
				},
			},
			wantNeedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService(tt.controlPlane)
			log := ctrl.Log.WithName("test")
			gotNeedUpdate, gotReq := s.checkDiffAndPrepareUpdateMaintenancePolicy(tt.existingCluster, &log)
			if gotNeedUpdate != tt.wantNeedUpdate {
				t.Errorf("checkDiffAndPrepareUpdateMaintenancePolicy() needUpdate = %v, want %v", gotNeedUpdate, tt.wantNeedUpdate)
			}
			if gotReq.GetName() != s.scope.ClusterFullName() {
				t.Errorf("checkDiffAndPrepareUpdateMaintenancePolicy() request name = %v, want %v", gotReq.GetName(), s.scope.ClusterFullName())
			}
			if gotNeedUpdate && gotReq.GetMaintenancePolicy().GetResourceVersion() != tt.existingCluster.GetMaintenancePolicy().GetResourceVersion() {
				t.Errorf("checkDiffAndPrepareUpdateMaintenancePolicy() did not carry forward existing ResourceVersion")
			}
		})
	}
}
