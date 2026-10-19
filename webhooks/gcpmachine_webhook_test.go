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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func TestGCPMachine_ValidateCreate(t *testing.T) {
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
		*infrav1.GCPMachine
		wantErr bool
	}{
		{
			name: "GCPMachined with OnHostMaintenance set to Terminate - valid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:      "n2d-standard-4",
					OnHostMaintenance: &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and OnHostMaintenance set to Terminate - valid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceTerminate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and OnHostMaintenance set to Migrate - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					OnHostMaintenance:   &onHostMaintenanceMigrate,
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and default OnHostMaintenance (Migrate) - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachined with ConfidentialCompute enabled and unsupported instance type - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeEnabled,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualization and supported instance type - valid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "c3d-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualization and unsupported instance type - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualization and OnHostMaintenance Migrate - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "c2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEV,
					OnHostMaintenance:   &onHostMaintenanceMigrate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and supported instance type - valid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and unsupported instance type - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "e2-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and OnHostMaintenance Migrate - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeSEVSNP,
					OnHostMaintenance:   &onHostMaintenanceMigrate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with ConfidentialCompute foobar - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "n2d-standard-4",
					ConfidentialCompute: &confidentialComputeFooBar,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with explicit TDX ConfidentialInstanceType and supported machine type - valid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "c3-standard-4",
					ConfidentialCompute: &confidentialComputeTDX,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with explicit TDX ConfidentialInstanceType and unsupported machine type - invalid",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					InstanceType:        "c3d-standard-4",
					ConfidentialCompute: &confidentialComputeTDX,
					OnHostMaintenance:   &onHostMaintenanceTerminate,
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Managed and Managed field set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
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
			name: "GCPMachine with RootDiskEncryptionKey KeyType Managed and Managed field not set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerManagedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and Supplied field not set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
					RootDiskEncryptionKey: &infrav1.CustomerEncryptionKey{
						KeyType: infrav1.CustomerSuppliedKey,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with AdditionalDisk Encryption KeyType Managed and Managed field not set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
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
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and one Supplied field set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
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
			name: "GCPMachine with RootDiskEncryptionKey KeyType Supplied and both Supplied fields set",
			GCPMachine: &infrav1.GCPMachine{
				Spec: infrav1.GCPMachineSpec{
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
			warn, err := (&GCPMachine{}).ValidateCreate(t.Context(), test.GCPMachine)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}

func TestGCPMachine_ResourcePoliciesImmutable(t *testing.T) {
	policy := "projects/test-project/regions/us-central1/resourcePolicies/spread"
	for _, tt := range []struct {
		name        string
		oldPolicies []string
		newPolicies []string
		wantErr     bool
	}{
		{name: "unchanged", oldPolicies: []string{policy}, newPolicies: []string{policy}},
		{name: "add", newPolicies: []string{policy}, wantErr: true},
		{name: "remove", oldPolicies: []string{policy}, wantErr: true},
		{name: "replace", oldPolicies: []string{policy}, newPolicies: []string{policy + "-other"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			oldMachine := &infrav1.GCPMachine{Spec: infrav1.GCPMachineSpec{ResourcePolicies: tt.oldPolicies}}
			newMachine := oldMachine.DeepCopy()
			newMachine.Spec.ResourcePolicies = tt.newPolicies
			_, err := (&GCPMachine{}).ValidateUpdate(t.Context(), oldMachine, newMachine)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestResourcePoliciesValidateCreate(t *testing.T) {
	const policy = "projects/my-project/regions/us-central1/resourcePolicies/spread"
	for _, tt := range []struct {
		name     string
		policies []string
		wantErr  bool
	}{
		{name: "omitted"},
		{name: "partial path", policies: []string{policy}},
		{name: "full URL", policies: []string{"https://www.googleapis.com/compute/v1/" + policy}},
		{name: "missing region", policies: []string{"projects/my-project/resourcePolicies/spread"}, wantErr: true},
		{name: "invalid name", policies: []string{policy + "_invalid"}, wantErr: true},
		{name: "wrong resource", policies: []string{"projects/my-project/regions/us-central1/subnetworks/spread"}, wantErr: true},
		{name: "missing project", policies: []string{"projects//regions/us-central1/resourcePolicies/spread"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			spec := infrav1.GCPMachineSpec{InstanceType: "n2-standard-2", ResourcePolicies: tt.policies}
			_, machineErr := (&GCPMachine{}).ValidateCreate(t.Context(), &infrav1.GCPMachine{Spec: spec})
			_, templateErr := (&GCPMachineTemplate{}).ValidateCreate(t.Context(), &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{Template: infrav1.GCPMachineTemplateResource{Spec: spec}},
			})
			for _, err := range []error{machineErr, templateErr} {
				if tt.wantErr {
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("resourcePolicies[0]"))
				} else {
					g.Expect(err).NotTo(HaveOccurred())
				}
			}
		})
	}
}
