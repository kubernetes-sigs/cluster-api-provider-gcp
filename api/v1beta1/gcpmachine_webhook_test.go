/*
Copyright 2023 The Kubernetes Authors.

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

func TestGCPMachine_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	confidentialComputeEnabled := ConfidentialComputePolicyEnabled
	confidentialComputeDisabled := ConfidentialComputePolicyDisabled
	foobar := ConfidentialVMTechnology("foobar")
	onHostMaintenanceTerminate := HostMaintenancePolicyTerminate
	onHostMaintenanceMigrate := HostMaintenancePolicyMigrate
	confidentialInstanceTypeSEV := ConfidentialVMTechnologySEV
	confidentialInstanceTypeSEVSNP := ConfidentialVMTechnologySEVSNP
	tests := []struct {
		name string
		*GCPMachine
		wantErr bool
	}{
		{
			name: "GCPMachined with OnHostMaintenance set to Terminate - valid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:      "n2d-standard-4",
					OnHostMaintenance: &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and OnHostMaintenance set to Terminate - valid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceTerminate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and OnHostMaintenance set to Migrate - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceMigrate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and default OnHostMaintenance (Migrate) - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and unsupported instance type - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with explicit ConfidentialInstanceType and ConfidentialCompute Disabled - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "n2d-standard-4",
					ConfidentialCompute:      &confidentialComputeDisabled,
					ConfidentialInstanceType: &confidentialInstanceTypeSEV,
					OnHostMaintenance:        &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with explicit ConfidentialInstanceType and OnHostMaintenance Migrate - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "n2d-standard-4",
					ConfidentialCompute:      &confidentialComputeEnabled,
					ConfidentialInstanceType: &confidentialInstanceTypeSEVSNP,
					OnHostMaintenance:        &onHostMaintenanceMigrate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with SEVSNP ConfidentialInstanceType and unsupported machine type - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "c2d-standard-4",
					ConfidentialCompute:      &confidentialComputeEnabled,
					ConfidentialInstanceType: &confidentialInstanceTypeSEVSNP,
					OnHostMaintenance:        &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with SEVSNP ConfidentialInstanceType and supported machine type - valid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "n2d-standard-4",
					ConfidentialCompute:      &confidentialComputeEnabled,
					ConfidentialInstanceType: &confidentialInstanceTypeSEVSNP,
					OnHostMaintenance:        &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with explicit SEV ConfidentialInstanceType and supported machine type - valid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "c3d-standard-4",
					ConfidentialCompute:      &confidentialComputeEnabled,
					ConfidentialInstanceType: &confidentialInstanceTypeSEV,
					OnHostMaintenance:        &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with unknown ConfidentialInstanceType and ConfidentialCompute Enabled - invalid",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					InstanceType:             "n2d-standard-4",
					ConfidentialCompute:      &confidentialComputeEnabled,
					ConfidentialInstanceType: &foobar,
					OnHostMaintenance:        &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Managed and Managed field set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					RootDiskEncryptionKey: &CustomerEncryptionKey{
						KeyType: CustomerManagedKey,
						ManagedKey: &ManagedKey{
							KMSKeyName: "projects/my-project/locations/us-central1/keyRings/us-central1/cryptoKeys/some-key",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Managed and Managed field not set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					RootDiskEncryptionKey: &CustomerEncryptionKey{
						KeyType: CustomerManagedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and Supplied field not set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					RootDiskEncryptionKey: &CustomerEncryptionKey{
						KeyType: CustomerSuppliedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with AdditionalDisk Encryption KeyType Managed and Managed field not set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					AdditionalDisks: []AttachedDiskSpec{
						{
							EncryptionKey: &CustomerEncryptionKey{
								KeyType: CustomerManagedKey,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and one Supplied field set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					RootDiskEncryptionKey: &CustomerEncryptionKey{
						KeyType: CustomerSuppliedKey,
						SuppliedKey: &SuppliedKey{
							RawKey: []byte("SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0="),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and both Supplied fields set",
			GCPMachine: &GCPMachine{
				Spec: GCPMachineSpec{
					RootDiskEncryptionKey: &CustomerEncryptionKey{
						KeyType: CustomerSuppliedKey,
						SuppliedKey: &SuppliedKey{
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
			warn, err := test.GCPMachine.ValidateCreate()
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}
