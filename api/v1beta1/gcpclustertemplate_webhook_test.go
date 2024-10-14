/*
Copyright 2021 The Kubernetes Authors.

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

	. "github.com/onsi/gomega"
)

func TestGCPClusterTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		newTemplate *GCPClusterTemplate
		oldTemplate *GCPClusterTemplate
		wantErr     bool
	}{
		{
			name: "GCPClusterTemplated with immutable spec",
			newTemplate: &GCPClusterTemplate{
				Spec: GCPClusterTemplateSpec{
					Template: GCPClusterTemplateResource{
						Spec: GCPClusterSpec{
							Project: "test-gcp-cluster",
							Region:  "ap-south-1",
						},
					},
				},
			},
			oldTemplate: &GCPClusterTemplate{
				Spec: GCPClusterTemplateSpec{
					Template: GCPClusterTemplateResource{
						Spec: GCPClusterSpec{
							Project: "test-gcp-cluster",
							Region:  "ap-south-1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPClusterTemplated with mutable spec",
			newTemplate: &GCPClusterTemplate{
				Spec: GCPClusterTemplateSpec{
					Template: GCPClusterTemplateResource{
						Spec: GCPClusterSpec{
							Project: "test-gcp-cluster",
							Region:  "ap-south-1",
						},
					},
				},
			},
			oldTemplate: &GCPClusterTemplate{
				Spec: GCPClusterTemplateSpec{
					Template: GCPClusterTemplateResource{
						Spec: GCPClusterSpec{
							Project: "test-gcp-cluster",
							Region:  "ap-east-1",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			warn, err := test.newTemplate.ValidateUpdate(test.oldTemplate)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}
