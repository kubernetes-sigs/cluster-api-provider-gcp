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
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

// on is shorthand for an add-on switched on with no options.
func on() infrav1exp.AddonSettings { return infrav1exp.AddonSettings{Enabled: true} }

// parse resolves config the way a caller would, failing the test if it doesn't make sense, so that tests
// about rendering aren't also tests about parsing.
func parse(t *testing.T, config infrav1exp.AddonsConfig) Config {
	t.Helper()
	resolved, errs := Parse(config, field.NewPath("spec", "addonsConfig"))
	if len(errs) > 0 {
		t.Fatalf("Parse() returned unexpected errors: %v", errs)
	}
	return resolved
}

// TestSupportedResolve is what lets Supported record only an add-on's name and leave where GKE keeps it
// to be discovered. Every key and option name has to still name a boolean in the GKE SDK, so that a bump
// renaming or moving one fails here rather than silently doing nothing to a real cluster.
func TestSupportedResolve(t *testing.T) {
	for _, addon := range Supported {
		addonsConfig := &containerpb.AddonsConfig{}

		message, enabled := resolveFlag(addonsConfig.ProtoReflect(), addon.Key, true)
		if enabled == nil {
			t.Errorf("add-on %q does not name a boolean in GKE's AddonsConfig", addon.Key)
			continue
		}

		for _, option := range addon.Options {
			if _, field := resolveFlag(message, option.Key, true); field == nil {
				t.Errorf("add-on %q option %q does not name a boolean in GKE's %s",
					addon.Key, option.Key, message.Descriptor().Name())
			}
		}
	}
}

func TestToGKE(t *testing.T) {
	tests := []struct {
		name   string
		config infrav1exp.AddonsConfig
		want   *containerpb.AddonsConfig
	}{
		{
			name:   "nothing configured",
			config: nil,
			want:   nil,
		},
		{
			name:   "an add-on switched on",
			config: infrav1exp.AddonsConfig{KeyDNSCache: on()},
			want:   &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
		{
			name:   "an add-on switched off is still sent, so that GKE turns it off",
			config: infrav1exp.AddonsConfig{KeyDNSCache: {}},
			want:   &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: false}},
		},
		{
			name: "an add-on with an option of its own",
			config: infrav1exp.AddonsConfig{
				KeyLustreCSIDriver: {Enabled: true, Options: map[string]bool{"disableMultiNic": true}},
			},
			want: &containerpb.AddonsConfig{
				LustreCsiDriverConfig: &containerpb.LustreCsiDriverConfig{Enabled: true, DisableMultiNic: true},
			},
		},
		{
			name: "add-ons the cluster said nothing about are left for GKE to default",
			config: infrav1exp.AddonsConfig{
				KeyGCSFuseCSIDriver: on(),
				KeyStatefulHA:       on(),
			},
			want: &containerpb.AddonsConfig{
				GcsFuseCsiDriverConfig: &containerpb.GcsFuseCsiDriverConfig{Enabled: true},
				StatefulHaConfig:       &containerpb.StatefulHAConfig{Enabled: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parse(t, test.config).ToGKE()
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ToGKE() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDiffGKE(t *testing.T) {
	tests := []struct {
		name        string
		config      infrav1exp.AddonsConfig
		existing    *containerpb.AddonsConfig
		wantChanged bool
		want        *containerpb.AddonsConfig
	}{
		{
			name:     "nothing configured changes nothing",
			config:   nil,
			existing: &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
		{
			name:     "an add-on already in the wanted state",
			config:   infrav1exp.AddonsConfig{KeyDNSCache: on()},
			existing: &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
		{
			name:        "an add-on that has to be switched on",
			config:      infrav1exp.AddonsConfig{KeyDNSCache: on()},
			existing:    &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: false}},
			wantChanged: true,
			want:        &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
		{
			name: "add-ons the cluster never mentioned are left alone",
			config: infrav1exp.AddonsConfig{
				KeyDNSCache: on(),
			},
			existing: &containerpb.AddonsConfig{
				DnsCacheConfig:         &containerpb.DnsCacheConfig{Enabled: false},
				GcsFuseCsiDriverConfig: &containerpb.GcsFuseCsiDriverConfig{Enabled: true},
			},
			wantChanged: true,
			want:        &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
		{
			// An add-on's enabled flag has no presence in proto3, so sending only the changed option
			// would read as switching the add-on off.
			name: "an add-on whose option changed is resent with its enabled flag",
			config: infrav1exp.AddonsConfig{
				KeyLustreCSIDriver: {Enabled: true, Options: map[string]bool{"disableMultiNic": true}},
			},
			existing: &containerpb.AddonsConfig{
				LustreCsiDriverConfig: &containerpb.LustreCsiDriverConfig{Enabled: true},
			},
			wantChanged: true,
			want: &containerpb.AddonsConfig{
				LustreCsiDriverConfig: &containerpb.LustreCsiDriverConfig{Enabled: true, DisableMultiNic: true},
			},
		},
		{
			name:   "an add-on that changed keeps the settings CAPG doesn't manage",
			config: infrav1exp.AddonsConfig{KeyRayOperator: on()},
			existing: &containerpb.AddonsConfig{
				RayOperatorConfig: &containerpb.RayOperatorConfig{
					RayClusterLoggingConfig: &containerpb.RayClusterLoggingConfig{Enabled: true},
				},
			},
			wantChanged: true,
			want: &containerpb.AddonsConfig{
				RayOperatorConfig: &containerpb.RayOperatorConfig{
					Enabled:                 true,
					RayClusterLoggingConfig: &containerpb.RayClusterLoggingConfig{Enabled: true},
				},
			},
		},
		{
			name:        "a cluster with no add-ons at all",
			config:      infrav1exp.AddonsConfig{KeyDNSCache: on()},
			existing:    nil,
			wantChanged: true,
			want:        &containerpb.AddonsConfig{DnsCacheConfig: &containerpb.DnsCacheConfig{Enabled: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed, got := parse(t, test.config).DiffGKE(test.existing)
			if changed != test.wantChanged {
				t.Errorf("DiffGKE() changed = %v, want %v", changed, test.wantChanged)
			}
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("DiffGKE() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		config    infrav1exp.AddonsConfig
		wantErrs  []string
		wantAddon map[string]bool
	}{
		{
			name:      "a recognized add-on",
			config:    infrav1exp.AddonsConfig{KeyDNSCache: on()},
			wantAddon: map[string]bool{KeyDNSCache: true},
		},
		{
			name:     "an add-on CAPG doesn't know",
			config:   infrav1exp.AddonsConfig{"dnsCacheConfg": on()},
			wantErrs: []string{"spec.addonsConfig[dnsCacheConfg]"},
		},
		{
			// GKE has this option, but CAPG doesn't support it yet.
			name: "an option CAPG doesn't support",
			config: infrav1exp.AddonsConfig{
				KeyRayOperator: {Enabled: true, Options: map[string]bool{"rayClusterLoggingConfig": true}},
			},
			wantErrs:  []string{"spec.addonsConfig[rayOperatorConfig].options[rayClusterLoggingConfig]"},
			wantAddon: map[string]bool{KeyRayOperator: true},
		},
		{
			name: "an option set on an add-on that is switched off",
			config: infrav1exp.AddonsConfig{
				KeyLustreCSIDriver: {Enabled: false, Options: map[string]bool{"disableMultiNic": true}},
			},
			wantErrs:  []string{"spec.addonsConfig[lustreCsiDriverConfig]"},
			wantAddon: map[string]bool{KeyLustreCSIDriver: false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, errs := Parse(test.config, field.NewPath("spec", "addonsConfig"))

			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}
			if diff := cmp.Diff(test.wantErrs, gotFields); diff != "" {
				t.Errorf("Parse() errors mismatch (-want +got):\n%s", diff)
			}

			for key, wantEnabled := range test.wantAddon {
				if got := config.Enabled(key); got != wantEnabled {
					t.Errorf("Enabled(%q) = %v, want %v", key, got, wantEnabled)
				}
			}
		})
	}
}

func TestCollectRequiredProjectAPIs(t *testing.T) {
	tests := []struct {
		name   string
		config infrav1exp.AddonsConfig
		want   []string
	}{
		{
			name:   "no add-on needs an API",
			config: infrav1exp.AddonsConfig{KeyDNSCache: on()},
		},
		{
			name:   "an add-on that needs one",
			config: infrav1exp.AddonsConfig{KeyAgentSandbox: on()},
			want:   []string{"artifactregistry.googleapis.com"},
		},
		{
			name:   "an add-on that needs one but is switched off",
			config: infrav1exp.AddonsConfig{KeyAgentSandbox: {}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CollectRequiredProjectAPIs(parse(t, test.config))
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("CollectRequiredProjectAPIs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
