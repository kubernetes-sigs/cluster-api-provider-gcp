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

package addons

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func controlPlaneWithPrometheus(monitoring *infrav1exp.MonitoringConfig) *infrav1exp.GCPManagedControlPlane {
	return &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{MonitoringConfig: monitoring},
		},
	}
}

func TestRayMonitoringOptionRequiresManagedPrometheus(t *testing.T) {
	rayWith := func(monitoring bool) infrav1exp.AddonsConfig {
		return infrav1exp.AddonsConfig{KeyRayOperator: {
			Enabled: true,
			Options: map[string]bool{OptionRayClusterMonitoringConfig: monitoring},
		}}
	}
	prometheus := func(enabled *bool) *infrav1exp.MonitoringConfig {
		return &infrav1exp.MonitoringConfig{ManagedPrometheusConfig: &infrav1exp.ManagedPrometheusConfig{Enabled: enabled}}
	}

	tests := []struct {
		name       string
		monitoring *infrav1exp.MonitoringConfig
		option     bool
		wantErr    bool
	}{
		{name: "no monitoringConfig at all", monitoring: nil, option: true},
		{name: "monitoringConfig without managedPrometheusConfig", monitoring: &infrav1exp.MonitoringConfig{}, option: true},
		{name: "managedPrometheusConfig left unset", monitoring: prometheus(nil), option: true},
		{name: "managedPrometheusConfig enabled", monitoring: prometheus(new(true)), option: true},
		{name: "managedPrometheusConfig disabled", monitoring: prometheus(new(false)), option: true, wantErr: true},
		{name: "option off means no check", monitoring: prometheus(new(false)), option: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := Intent{
				ControlPlane: controlPlaneWithPrometheus(test.monitoring),
				Config:       parse(t, rayWith(test.option)),
			}
			violations := Validate(intent)
			if got := len(violations) > 0; got != test.wantErr {
				t.Fatalf("Validate() = %v, want an error: %v", violations, test.wantErr)
			}
			if test.wantErr && !strings.Contains(violations[0].Message, OptionRayClusterMonitoringConfig) {
				t.Errorf("violation %q should name the option", violations[0].Message)
			}
		})
	}
}

// Validate must not change the Config it is given: ToGKE reads the same options afterwards.
func TestValidateLeavesConfigAlone(t *testing.T) {
	config := parse(t, infrav1exp.AddonsConfig{KeyRayOperator: {
		Enabled: true,
		Options: map[string]bool{OptionRayClusterMonitoringConfig: false},
	}})
	before := config.ToGKE()

	Validate(Intent{ControlPlane: controlPlaneWithPrometheus(nil), Config: config})

	if diff := cmp.Diff(before.String(), config.ToGKE().String()); diff != "" {
		t.Errorf("Validate() changed the config (-before +after):\n%s", diff)
	}
}

func TestRequiresGKEMetadataMode(t *testing.T) {
	pools := func(modes ...string) []NodePoolState {
		states := make([]NodePoolState, len(modes))
		for i, mode := range modes {
			states[i] = NodePoolState{Name: "pool-" + string(rune('0'+i)), WorkloadMetadataMode: mode}
		}
		return states
	}

	tests := []struct {
		name     string
		pools    []NodePoolState
		wantPool string // empty: no error expected
	}{
		{name: "every pool uses GKE_METADATA", pools: pools("GKE_METADATA", "GKE_METADATA")},
		{name: "one pool uses GCE_METADATA", pools: pools("GKE_METADATA", "GCE_METADATA"), wantPool: "pool-1"},
		{name: "pools GKE hasn't resolved are skipped", pools: pools("", "")},
		{name: "unresolved beside GKE_METADATA", pools: pools("", "GKE_METADATA")},
		{name: "no pools", pools: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := RequiresGKEMetadataMode().(ClusterConstraint).CheckCluster(Snapshot{NodePools: test.pools})
			switch {
			case test.wantPool == "" && err != nil:
				t.Errorf("CheckCluster() = %v, want nil", err)
			case test.wantPool != "" && (err == nil || !strings.Contains(err.Error(), test.wantPool)):
				t.Errorf("CheckCluster() = %v, want an error naming %q", err, test.wantPool)
			}
		})
	}
}

// The constraint is on GCS Fuse and nowhere else: it once landed on Stateful HA by mistake.
func TestGKEMetadataModeConstraintIsOnGCSFuseOnly(t *testing.T) {
	for _, addon := range Supported {
		has := false
		for _, constraint := range addon.ClusterConstraints {
			if _, ok := constraint.(requiresGKEMetadataMode); ok {
				has = true
			}
		}
		if want := addon.Key == KeyGCSFuseCSIDriver; has != want {
			t.Errorf("add-on %q has the GKE metadata constraint = %v, want %v", addon.Key, has, want)
		}
	}
}
