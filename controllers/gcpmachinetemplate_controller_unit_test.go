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

package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onsi/gomega"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestMachineDeploymentAndMachineSetToGCPMachineTemplate(t *testing.T) {
	g := gomega.NewWithT(t)

	md := &clusterv1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{Name: "workers", Namespace: "default"}}
	md.Spec.Template.Spec.InfrastructureRef.Kind = "GCPMachineTemplate"
	md.Spec.Template.Spec.InfrastructureRef.Name = "worker-template"
	ms := &clusterv1.MachineSet{ObjectMeta: metav1.ObjectMeta{Name: "workers-set", Namespace: "default"}}
	ms.Spec.Template.Spec.InfrastructureRef.Kind = "GCPMachineTemplate"
	ms.Spec.Template.Spec.InfrastructureRef.Name = "worker-template"
	expected := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "worker-template"}}

	g.Expect(machineDeploymentToGCPMachineTemplate(context.Background(), md)).To(gomega.ConsistOf(expected))
	g.Expect(machineSetToGCPMachineTemplate(context.Background(), ms)).To(gomega.ConsistOf(expected))
	g.Expect(machineDeploymentToGCPMachineTemplate(context.Background(), &clusterv1.MachineDeployment{})).To(gomega.BeEmpty())
}

func TestGetMachineType(t *testing.T) {
	g := gomega.NewWithT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(gomega.Equal("/projects/test-project/zones/us-central1-a/machineTypes/n2-standard-4"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"guestCpus":4,"memoryMb":16384,"architecture":"X86_64"}`))
	}))
	defer server.Close()

	service, err := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
	g.Expect(err).NotTo(gomega.HaveOccurred())
	machineType, err := getMachineType(context.Background(), service, "test-project", "us-central1-a", "n2-standard-4")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(machineType.GuestCpus).To(gomega.Equal(int64(4)))
	g.Expect(machineType.MemoryMb).To(gomega.Equal(int64(16384)))
	g.Expect(machineType.Architecture).To(gomega.Equal("X86_64"))
}

func TestMachineTypeCapacity(t *testing.T) {
	g := gomega.NewWithT(t)

	capacity, err := machineTypeCapacity(&compute.MachineType{GuestCpus: 4, MemoryMb: 16384})

	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(capacity.Cpu().String()).To(gomega.Equal("4"))
	g.Expect(capacity.Memory().String()).To(gomega.Equal("16Gi"))
}

func TestMachineTypeCapacityRejectsInvalidValues(t *testing.T) {
	g := gomega.NewWithT(t)

	_, err := machineTypeCapacity(&compute.MachineType{GuestCpus: 0, MemoryMb: 16384})

	g.Expect(err).To(gomega.HaveOccurred())
}

func TestGetArchitectureFromMachineType(t *testing.T) {
	tests := []struct {
		name         string
		machineType  *compute.MachineType
		expectedArch string
	}{
		{
			name:         "x86_64 architecture",
			machineType:  &compute.MachineType{Name: "n2-standard-4", Architecture: "X86_64"},
			expectedArch: "amd64",
		},
		{
			name:         "ARM64 architecture",
			machineType:  &compute.MachineType{Name: "t2a-standard-4", Architecture: "ARM64"},
			expectedArch: "arm64",
		},
		{
			name:         "empty architecture defaults to amd64",
			machineType:  &compute.MachineType{Name: "n2-standard-4", Architecture: ""},
			expectedArch: "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			arch := getArchitectureFromMachineType(tt.machineType)
			g.Expect(string(arch)).To(gomega.Equal(tt.expectedArch))
		})
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name           string
		ref            string
		defaultProject string
		wantProject    string
		wantName       string
		wantError      bool
	}{
		{
			name:           "full path with image",
			ref:            "projects/debian-cloud/global/images/debian-11-bullseye-v20260101",
			defaultProject: "my-project",
			wantProject:    "debian-cloud",
			wantName:       "debian-11-bullseye-v20260101",
		},
		{
			name:           "full path with family",
			ref:            "projects/debian-cloud/global/images/family/debian-11",
			defaultProject: "my-project",
			wantProject:    "debian-cloud",
			wantName:       "debian-11",
		},
		{
			name:           "global path",
			ref:            "global/images/my-image",
			defaultProject: "my-project",
			wantProject:    "my-project",
			wantName:       "my-image",
		},
		{
			name:           "short form image",
			ref:            "debian-11-bullseye-v20260101",
			defaultProject: "my-project",
			wantProject:    "my-project",
			wantName:       "debian-11-bullseye-v20260101",
		},
		{
			name:           "short form family",
			ref:            "family/debian-11",
			defaultProject: "my-project",
			wantProject:    "my-project",
			wantName:       "debian-11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			project, name, err := parseImageReference(tt.ref, tt.defaultProject)

			if tt.wantError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(project).To(gomega.Equal(tt.wantProject))
				g.Expect(name).To(gomega.Equal(tt.wantName))
			}
		})
	}
}

func TestMachineTypeNodeInfoWithImages(t *testing.T) {
	tests := []struct {
		name          string
		imageRef      string
		imageFamily   string
		imageResponse string
		machineArch   string
		expectedArch  string
		expectedOS    string
		expectError   bool
	}{
		{
			name:     "Linux x86_64 image",
			imageRef: "projects/debian-cloud/global/images/debian-11-bullseye-v20260101",
			imageResponse: `{
				"name": "debian-11-bullseye-v20260101",
				"guestOsFeatures": [{"type": "VIRTIO_SCSI_MULTIQUEUE"}, {"type": "UEFI_COMPATIBLE"}]
			}`,
			machineArch:  "X86_64",
			expectedArch: "amd64",
			expectedOS:   "linux",
		},
		{
			name:     "Windows x86_64 image",
			imageRef: "projects/windows-cloud/global/images/windows-server-2022-dc-v20260101",
			imageResponse: `{
				"name": "windows-server-2022-dc-v20260101",
				"guestOsFeatures": [{"type": "WINDOWS"}, {"type": "UEFI_COMPATIBLE"}]
			}`,
			machineArch:  "X86_64",
			expectedArch: "amd64",
			expectedOS:   "windows",
		},
		{
			name:        "Linux ARM64 image family",
			imageFamily: "debian-11-arm64",
			imageResponse: `{
				"name": "debian-11-arm64-v20260101",
				"architecture": "ARM64",
				"guestOsFeatures": [{"type": "UEFI_COMPATIBLE"}]
			}`,
			machineArch:  "ARM64",
			expectedArch: "arm64",
			expectedOS:   "linux",
		},
		{
			name:         "No image specified defaults to Linux",
			machineArch:  "X86_64",
			expectedArch: "amd64",
			expectedOS:   "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			// Mock server for Images API
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.imageResponse != "" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.imageResponse))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			service, err := compute.NewService(context.Background(),
				option.WithEndpoint(server.URL), option.WithoutAuthentication())
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Create template with image reference
			template := &infrav1.GCPMachineTemplate{}
			if tt.imageRef != "" {
				template.Spec.Template.Spec.Image = &tt.imageRef
			} else if tt.imageFamily != "" {
				template.Spec.Template.Spec.ImageFamily = &tt.imageFamily
			}

			// Create machine type
			machineType := &compute.MachineType{
				Name:         "n2-standard-4",
				GuestCpus:    4,
				MemoryMb:     16384,
				Architecture: tt.machineArch,
			}

			nodeInfo, err := machineTypeNodeInfo(context.Background(), service,
				"test-project", machineType, template)

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(string(nodeInfo.Architecture)).To(gomega.Equal(tt.expectedArch))
				g.Expect(string(nodeInfo.OperatingSystem)).To(gomega.Equal(tt.expectedOS))
			}
		})
	}
}
