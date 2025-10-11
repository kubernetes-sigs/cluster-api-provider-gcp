/*
Copyright 2025 The Kubernetes Authors.

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
	"context"
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestGCPMachine_AliasIPRanges_Validation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Setup the test environment with CRDs
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())

	defer func() {
		err := testEnv.Stop()
		g.Expect(err).NotTo(HaveOccurred())
	}()

	err = AddToScheme(scheme.Scheme)
	g.Expect(err).NotTo(HaveOccurred())

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(k8sClient).NotTo(BeNil())

	namespace := "default"

	tests := []struct {
		name          string
		aliasIPRanges []AliasIPRange
		wantErr       bool
		errorContains string
	}{
		// Valid cases - these should be accepted
		{
			name: "valid CIDR notation",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid IP address only",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid netmask only",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid without subnetwork range name",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange: "/24",
				},
			},
			wantErr: false,
		},
		{
			name: "valid multiple ranges",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "10.0.0.0/24",
					SubnetworkRangeName: "pods",
				},
				{
					IPCidrRange:         "10.1.0.0/24",
					SubnetworkRangeName: "services",
				},
			},
			wantErr: false,
		},
		{
			name:          "valid empty alias IP ranges",
			aliasIPRanges: []AliasIPRange{},
			wantErr:       false,
		},
		{
			name:          "valid nil alias IP ranges",
			aliasIPRanges: nil,
			wantErr:       false,
		},
		// Invalid cases - these should be rejected by CRD validation
		{
			name: "invalid netmask too large",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "/33",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid empty ipCidrRange",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid IP address out of range",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "1270.0.0.1/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid IP address with letters",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1a",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid CIDR with letters",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1a/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid format with extra slash",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "10.0.0.0//24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
		{
			name: "invalid format with space",
			aliasIPRanges: []AliasIPRange{
				{
					IPCidrRange:         "10.0.0.0 /24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr:       true,
			errorContains: "should match",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a GCPMachine with the test aliasIPRanges
			machine := &GCPMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("test-machine-%d", i),
					Namespace: namespace,
				},
				Spec: GCPMachineSpec{
					InstanceType:  "n1-standard-2",
					AliasIPRanges: tt.aliasIPRanges,
				},
			}

			// Attempt to create the machine
			err := k8sClient.Create(ctx, machine)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorContains))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				// Clean up successfully created resources
				_ = k8sClient.Delete(ctx, machine)
			}
		})
	}
}
