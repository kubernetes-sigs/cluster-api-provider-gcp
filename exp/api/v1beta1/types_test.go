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
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"k8s.io/utils/ptr"
)

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
