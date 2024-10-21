/*
Copyright 2024 The Kubernetes Authors.

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
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestGCPManagedClusterValidatingWebhookUpdate(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		spec        GCPManagedClusterSpec
	}{
		{
			name:        "request to change mutable field additional labels",
			expectError: false,
			spec: GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-west1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
				AdditionalLabels: map[string]string{
					"testKey": "testVal",
				},
			},
		},
		{
			name:        "request to change immutable field project",
			expectError: true,
			spec: GCPManagedClusterSpec{
				Project: "new-project",
				Region:  "us-west1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
			},
		},
		{
			name:        "request to change immutable field region",
			expectError: true,
			spec: GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-central1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "credsref",
				},
			},
		},
		{
			name:        "request to change immutable field credentials ref",
			expectError: true,
			spec: GCPManagedClusterSpec{
				Project: "old-project",
				Region:  "us-central1",
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "new-ns",
					Name:      "new-name",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			newMC := &GCPManagedCluster{
				Spec: tc.spec,
			}
			oldMC := &GCPManagedCluster{
				Spec: GCPManagedClusterSpec{
					Project: "old-project",
					Region:  "us-west1",
					CredentialsRef: &infrav1.ObjectReference{
						Namespace: "default",
						Name:      "credsref",
					},
				},
			}

			warn, err := newMC.ValidateUpdate(oldMC)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			// Nothing emits warnings yet
			g.Expect(warn).To(BeEmpty())
		})
	}
}
