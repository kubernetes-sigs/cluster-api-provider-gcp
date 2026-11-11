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

package v1beta1

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func getEnumVariants(t *testing.T, file, typeName string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE || gen.Doc == nil {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				continue
			}
			for _, c := range gen.Doc.List {
				if v, ok := strings.CutPrefix(c.Text, "// +kubebuilder:validation:Enum="); ok {
					return strings.Split(v, ";")
				}
			}
		}
	}
	t.Fatalf("no Enum marker on type %s in %s", typeName, file)
	return nil
}

func TestConvertToSdkWorkloadMetadataModeCoversEveryEnumVariant(t *testing.T) {
	enumVariants := getEnumVariants(t, "gcpmanagedmachinepool_types.go", "WorkloadMetadataMode")
	for _, variant := range enumVariants {
		if ConvertToSdkWorkloadMetadataMode(new(WorkloadMetadataMode(variant))) == nil {
			t.Errorf("ConvertToSdkWorkloadMetadataMode(%v) should not return nil as enum variants "+
				"should match the SDK", variant)
		}
	}
}

func TestConvertToSdkWorkloadMetadataMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     *WorkloadMetadataMode
		wantMode *containerpb.WorkloadMetadataConfig_Mode
	}{
		{
			name:     "nil returns nil",
			mode:     nil,
			wantMode: nil,
		},
		{
			name:     "GKE_METADATA maps to GKE_METADATA",
			mode:     ptr.To(WorkloadMetadataModeGKEMetadata),
			wantMode: ptr.To(containerpb.WorkloadMetadataConfig_GKE_METADATA),
		},
		{
			name:     "GCE_METADATA maps to GCE_METADATA",
			mode:     ptr.To(WorkloadMetadataModeGCEMetadata),
			wantMode: ptr.To(containerpb.WorkloadMetadataConfig_GCE_METADATA),
		},
		{
			name:     "unknown mode returns nil",
			mode:     ptr.To(WorkloadMetadataMode("UNKNOWN")),
			wantMode: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ConvertToSdkWorkloadMetadataMode(tc.mode)
			if tc.wantMode == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else if got.GetMode() != *tc.wantMode {
				t.Errorf("expected mode %v, got %v", *tc.wantMode, got.GetMode())
			}
		})
	}
}

func TestConvertToSdkTaintConfigCoversEveryEnumVariant(t *testing.T) {
	enumVariants := getEnumVariants(t, "gcpmanagedmachinepool_types.go", "ArchitectureTaintBehavior")
	for _, variant := range enumVariants {
		if ConvertToSdkTaintConfig(new(ArchitectureTaintBehavior(variant))) == nil {
			t.Errorf("ConvertToSdkTaintConfig(%v) should not return nil as enum variants "+
				"should match the SDK", variant)
		}
	}
}

func TestConvertToSdkTaintConfig(t *testing.T) {
	tests := []struct {
		name         string
		behavior     *ArchitectureTaintBehavior
		wantBehavior *containerpb.TaintConfig_ArchitectureTaintBehavior
	}{
		{
			name:         "nil returns nil",
			behavior:     nil,
			wantBehavior: nil,
		},
		{
			name:         "none maps to NONE",
			behavior:     ptr.To(ArchitectureTaintBehaviorNone),
			wantBehavior: ptr.To(containerpb.TaintConfig_NONE),
		},
		{
			name:         "arm maps to ARM",
			behavior:     ptr.To(ArchitectureTaintBehaviorARM),
			wantBehavior: ptr.To(containerpb.TaintConfig_ARM),
		},
		{
			name:         "unknown behavior returns nil",
			behavior:     ptr.To(ArchitectureTaintBehavior("unknown")),
			wantBehavior: nil,
		},
		{
			name:         "unspecified is not reachable through the API",
			behavior:     ptr.To(ArchitectureTaintBehavior("architecture_taint_behavior_unspecified")),
			wantBehavior: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ConvertToSdkTaintConfig(tc.behavior)
			if tc.wantBehavior == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else if got.GetArchitectureTaintBehavior() != *tc.wantBehavior {
				t.Errorf("expected behavior %v, got %v", *tc.wantBehavior, got.GetArchitectureTaintBehavior())
			}
		})
	}
}

func TestConvertToSdkReservationAffinityCoversEveryEnumVariant(t *testing.T) {
	enumVariants := getEnumVariants(t, "../../../api/v1beta1/gcpmachine_types.go", "ReservationConsumeType")
	for _, variant := range enumVariants {
		if ConvertToSdkReservationAffinity(&infrav1.ReservationAffinity{ConsumeReservationType: infrav1.ReservationConsumeType(variant)}) == nil {
			t.Errorf("ConvertToSdkReservationAffinity(%v) should not return nil as enum variants "+
				"should match the SDK", variant)
		}
	}
}

func TestConvertToSdkReservationAffinity(t *testing.T) {
	tests := []struct {
		name         string
		affinity     *infrav1.ReservationAffinity
		wantAffinity *containerpb.ReservationAffinity
	}{
		{
			name:         "nil returns nil",
			affinity:     nil,
			wantAffinity: nil,
		},
		{
			name: "unspecified is unreachable",
			affinity: &infrav1.ReservationAffinity{
				ConsumeReservationType: "UNSPECIFIED",
			},
			wantAffinity: nil,
		},
		{
			name: "any reservation passes through",
			affinity: &infrav1.ReservationAffinity{
				ConsumeReservationType: "ANY_RESERVATION",
			},
			wantAffinity: &containerpb.ReservationAffinity{
				ConsumeReservationType: containerpb.ReservationAffinity_ANY_RESERVATION,
			},
		},
		{
			name: "specific reservation saves keys and values",
			affinity: &infrav1.ReservationAffinity{
				ConsumeReservationType: "SPECIFIC_RESERVATION",
				Key:                    "foo",
				Values:                 []string{"bar", "bing", "bash"},
			},
			wantAffinity: &containerpb.ReservationAffinity{
				ConsumeReservationType: containerpb.ReservationAffinity_SPECIFIC_RESERVATION,
				Key:                    "foo",
				Values:                 []string{"bar", "bing", "bash"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ConvertToSdkReservationAffinity(tc.affinity)
			if tc.wantAffinity == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else if got.GetConsumeReservationType() != tc.wantAffinity.GetConsumeReservationType() || got.GetKey() != tc.wantAffinity.GetKey() || !slices.Equal(got.GetValues(), tc.wantAffinity.GetValues()) {
				t.Errorf("expected affinity %v, got %v", tc.wantAffinity, got)
			}
		})
	}
}
