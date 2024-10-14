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

func TestGCPCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name       string
		newCluster *GCPCluster
		oldCluster *GCPCluster
		wantErr    bool
	}{
		{
			name: "GCPCluster with MTU field is within the limits of more than 1300 and less than 8896",
			newCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			oldCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(1400),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPCluster with MTU field more than 8896",
			newCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(10000),
					},
				},
			},
			oldCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPCluster with MTU field less than 8896",
			newCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(1250),
					},
				},
			},
			oldCluster: &GCPCluster{
				Spec: GCPClusterSpec{
					Network: NetworkSpec{
						Mtu: int64(1500),
					},
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			warn, err := test.newCluster.ValidateUpdate(test.oldCluster)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}
