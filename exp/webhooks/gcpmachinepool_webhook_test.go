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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func TestGCPMachinePool_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	confidentialComputeEnabled := infrav1.ConfidentialComputePolicyEnabled
	confidentialComputeSEV := infrav1.ConfidentialComputePolicySEV
	confidentialComputeSEVSNP := infrav1.ConfidentialComputePolicySEVSNP
	confidentialComputeTDX := infrav1.ConfidentialComputePolicyTDX
	confidentialComputeFooBar := infrav1.ConfidentialComputePolicy("foobar")
	onHostMaintenanceTerminate := infrav1.HostMaintenancePolicyTerminate
	onHostMaintenanceMigrate := infrav1.HostMaintenancePolicyMigrate
	tests := []struct {
		name string
		*expinfrav1.GCPMachinePool
		wantErr bool
	}{
		{
			name: "GCPMachinePool with OnHostMaintenance set to Terminate - valid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:      "n2d-standard-4",
					OnHostMaintenance: &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute enabled and OnHostMaintenance set to Terminate - valid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceTerminate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute enabled and OnHostMaintenance set to Migrate - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceMigrate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute enabled and default OnHostMaintenance (Migrate) - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute enabled and unsupported instance type - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualization and supported instance type - valid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "c3d-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualization and unsupported instance type - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualization and OnHostMaintenance Migrate - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "c2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceMigrate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and supported instance type - valid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and unsupported instance type - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and OnHostMaintenance Migrate - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceMigrate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with ConfidentialCompute foobar - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeFooBar,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with explicit TDX ConfidentialInstanceType and supported machine type - valid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "c3-standard-4",
					ConfidentialCompute: &confidentialComputeTDX,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with explicit TDX ConfidentialInstanceType and unsupported machine type - invalid",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "c3d-standard-4",
					ConfidentialCompute: &confidentialComputeTDX,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with RootDiskEncryptionKey KeyType Managed and Managed field set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerManagedKey,
						ManagedKey: &infrav1.ManagedKey{
							KMSKeyName: "projects/my-project/locations/us-central1/keyRings/us-central1/cryptoKeys/some-key",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with RootDiskEncryptionKey KeyType Managed and Managed field not set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerManagedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with RootDiskEncryptionKey KeyType Supplied and Supplied field not set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerSuppliedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with AdditionalDisk Encryption KeyType Managed and Managed field not set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					AdditionalDisks: []infrav1.AttachedDiskSpec{
						{
							EncryptionKey: &infrav1.CustomerEncryptionKey{
								KeyType: infrav1.CustomerManagedKey,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachinePool with RootDiskEncryptionKey KeyType Supplied and one Supplied field set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerSuppliedKey,
						SuppliedKey: &infrav1.SuppliedKey{
							RawKey: []byte("SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0="),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachinePool with RootDiskEncryptionKey KeyType Supplied and both Supplied fields set",
			GCPMachinePool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerSuppliedKey,
						SuppliedKey: &infrav1.SuppliedKey{
							RawKey:          []byte("SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0="),
							RSAEncryptedKey: []byte("SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0="),
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
			warn, err := (&GCPMachinePool{}).ValidateCreate(t.Context(), test.GCPMachinePool)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}

func TestGCPMachinePool_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	oldPool := &expinfrav1.GCPMachinePool{
		Spec: expinfrav1.GCPMachinePoolSpec{
			InstanceType:     "n2d-standard-4",
			ProviderIDList:   []string{"gce://project/zone/instance-1"},
			AdditionalLabels: infrav1.Labels{"foo": "bar"},
		},
	}

	tests := []struct {
		name    string
		newPool *expinfrav1.GCPMachinePool
		wantErr bool
	}{
		{
			name: "allow changes to providerIDList",
			newPool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:     "n2d-standard-4",
					ProviderIDList:   []string{"gce://project/zone/instance-1", "gce://project/zone/instance-2"},
					AdditionalLabels: infrav1.Labels{"foo": "bar"},
				},
			},
			wantErr: false,
		},
		{
			name: "allow changes to additionalLabels",
			newPool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:     "n2d-standard-4",
					ProviderIDList:   []string{"gce://project/zone/instance-1"},
					AdditionalLabels: infrav1.Labels{"foo": "baz"},
				},
			},
			wantErr: false,
		},
		{
			name: "allow changes to additionalNetworkTags",
			newPool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:          "n2d-standard-4",
					ProviderIDList:        []string{"gce://project/zone/instance-1"},
					AdditionalLabels:      infrav1.Labels{"foo": "bar"},
					AdditionalNetworkTags: []string{"new-tag"},
				},
			},
			wantErr: false,
		},
		{
			name: "reject update with invalid ConfidentialCompute configuration",
			newPool: &expinfrav1.GCPMachinePool{
				Spec: expinfrav1.GCPMachinePoolSpec{
					InstanceType:        "e2-standard-4", // e2 does not support SEV
					ConfidentialCompute: &(&struct{ p infrav1.ConfidentialComputePolicy }{infrav1.ConfidentialComputePolicySEV}).p,
					ProviderIDList:      []string{"gce://project/zone/instance-1"},
					AdditionalLabels:    infrav1.Labels{"foo": "bar"},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			warn, err := (&GCPMachinePool{}).ValidateUpdate(t.Context(), oldPool, test.newPool)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}
