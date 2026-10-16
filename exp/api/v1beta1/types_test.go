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
	"strings"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"k8s.io/utils/ptr"
)

func TestConvertToSdkWorkloadMetadataModeCoversEveryEnumVariant(t *testing.T) {
	enumVariants := getEnumVariants(t, "gcpmanagedmachinepool_types.go", "WorkloadMetadataMode")
	for _, variant := range enumVariants {
		if ConvertToSdkWorkloadMetadataMode(new(WorkloadMetadataMode(variant))) == nil {
			t.Errorf("ConvertToSdkWorkloadMetadataMode(%v) should not return nil as enum variants "+
				"should match the SDK", variant)
		}
	}
}

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
