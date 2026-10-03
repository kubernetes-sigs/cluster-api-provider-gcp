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

package controllers

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var _ = Describe("GCPMachine AliasIPRanges validation", func() {
	tests := []struct {
		name          string
		aliasIPRanges []infrav1.AliasIPRange
		wantErr       bool
		errorContains string
	}{
		// Valid cases - these should be accepted
		{
			name: "valid CIDR notation",
			aliasIPRanges: []infrav1.AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid IP address only",
			aliasIPRanges: []infrav1.AliasIPRange{
				{
					IPCidrRange:         "127.0.0.1",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid netmask only",
			aliasIPRanges: []infrav1.AliasIPRange{
				{
					IPCidrRange:         "/24",
					SubnetworkRangeName: "subnet-name",
				},
			},
			wantErr: false,
		},
		{
			name: "valid without subnetwork range name",
			aliasIPRanges: []infrav1.AliasIPRange{
				{
					IPCidrRange: "/24",
				},
			},
			wantErr: false,
		},
		{
			name: "valid multiple ranges",
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{},
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
			aliasIPRanges: []infrav1.AliasIPRange{
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
		It(tt.name, func() {
			ctx := context.Background()

			By("Creating a GCPMachine with the test aliasIPRanges")
			machine := &infrav1.GCPMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("test-machine-aliasipranges-%d", i),
					Namespace: "default",
				},
				Spec: infrav1.GCPMachineSpec{
					InstanceType:  "n1-standard-2",
					AliasIPRanges: tt.aliasIPRanges,
				},
			}

			err := k8sClient.Create(ctx, machine)

			if tt.wantErr {
				Expect(err).To(HaveOccurred())
				if tt.errorContains != "" {
					Expect(err.Error()).To(ContainSubstring(tt.errorContains))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
				// Clean up successfully created resources
				Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
			}
		})
	}
})
