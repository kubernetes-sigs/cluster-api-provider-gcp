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

func TestGCPMachineTemplate_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	confidentialComputeEnabled := infrav1.ConfidentialComputePolicyEnabled
	confidentialComputeSEV := infrav1.ConfidentialComputePolicySEV
	confidentialComputeSEVSNP := infrav1.ConfidentialComputePolicySEVSNP
	confidentialComputeTDX := infrav1.ConfidentialComputePolicyTDX
	onHostMaintenanceTerminate := infrav1.HostMaintenancePolicyTerminate
	onHostMaintenanceMigrate := infrav1.HostMaintenancePolicyMigrate
	tests := []struct {
		name     string
		template *infrav1.GCPMachineTemplate
		wantErr  bool
	}{
		{
			name: "GCPMachineTemplate with OnHostMaintenance set to Terminate - valid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:      "n2d-standard-4",
							OnHostMaintenance: &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute enabled and OnHostMaintenance set to Terminate - valid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "n2d-standard-4",
							ConfidentialCompute: &confidentialComputeEnabled,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute enabled and OnHostMaintenance set to Migrate - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "n2d-standard-4",
							ConfidentialCompute: &confidentialComputeEnabled,
							OnHostMaintenance:   &onHostMaintenanceMigrate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute enabled and default OnHostMaintenance (Migrate) - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "n2d-standard-4",
							ConfidentialCompute: &confidentialComputeEnabled,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute enabled and unsupported instance type - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "e2-standard-4",
							ConfidentialCompute: &confidentialComputeEnabled,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualization and unsupported instance type - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "e2-standard-4",
							ConfidentialCompute: &confidentialComputeSEV,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualization and supported instance type - valid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c2d-standard-4",
							ConfidentialCompute: &confidentialComputeSEV,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualization and OnHostMaintenance Migrate - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c2d-standard-4",
							ConfidentialCompute: &confidentialComputeSEV,
							OnHostMaintenance:   &onHostMaintenanceMigrate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and unsupported instance type - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c2d-standard-4",
							ConfidentialCompute: &confidentialComputeSEVSNP,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and supported instance type - valid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "n2d-standard-4",
							ConfidentialCompute: &confidentialComputeSEVSNP,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachineTemplate with ConfidentialCompute AMDEncryptedVirtualizationNestedPaging and OnHostMaintenance Migrate - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c2d-standard-4",
							ConfidentialCompute: &confidentialComputeSEVSNP,
							OnHostMaintenance:   &onHostMaintenanceMigrate,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "GCPMachine with explicit TDX ConfidentialInstanceType and supported machine type - valid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c3-standard-4",
							ConfidentialCompute: &confidentialComputeTDX,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GCPMachine with explicit TDX ConfidentialInstanceType and unsupported machine type - invalid",
			template: &infrav1.GCPMachineTemplate{
				Spec: infrav1.GCPMachineTemplateSpec{
					Template: infrav1.GCPMachineTemplateResource{
						Spec: infrav1.GCPMachineSpec{
							InstanceType:        "c3d-standard-4",
							ConfidentialCompute: &confidentialComputeTDX,
							OnHostMaintenance:   &onHostMaintenanceTerminate,
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
			warn, err := (&GCPMachineTemplate{}).ValidateCreate(t.Context(), test.template)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warn).To(BeNil())
		})
	}
}
