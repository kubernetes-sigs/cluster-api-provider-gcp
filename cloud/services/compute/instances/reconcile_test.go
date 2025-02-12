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

package instances

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1.AddToScheme(scheme.Scheme)
}

var fakeBootstrapSecret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster-bootstrap",
		Namespace: "default",
	},
	Data: map[string][]byte{
		"value": []byte("Zm9vCg=="),
	},
}

var fakeCluster = &clusterv1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: clusterv1.ClusterSpec{},
}

var fakeMachine = &clusterv1.Machine{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-machine",
		Namespace: "default",
	},
	Spec: clusterv1.MachineSpec{
		Bootstrap: clusterv1.Bootstrap{
			DataSecretName: ptr.To[string]("my-cluster-bootstrap"),
		},
		FailureDomain: ptr.To[string]("us-central1-c"),
		Version:       ptr.To[string]("v1.19.11"),
	},
}

var fakeMachineWithOutFailureDomain = &clusterv1.Machine{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-machine",
		Namespace: "default",
	},
	Spec: clusterv1.MachineSpec{
		Bootstrap: clusterv1.Bootstrap{
			DataSecretName: ptr.To[string]("my-cluster-bootstrap"),
		},
		Version: ptr.To[string]("v1.19.11"),
	},
}

var fakeGCPClusterWithOutFailureDomain = &infrav1.GCPCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: infrav1.GCPClusterSpec{
		Project: "my-proj",
		Region:  "us-central1",
	},
	Status: infrav1.GCPClusterStatus{
		FailureDomains: clusterv1.FailureDomains{
			"us-central1-a": clusterv1.FailureDomainSpec{ControlPlane: true},
			"us-central1-b": clusterv1.FailureDomainSpec{ControlPlane: true},
			"us-central1-c": clusterv1.FailureDomainSpec{ControlPlane: true},
		},
	},
}

var fakeGCPCluster = &infrav1.GCPCluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-cluster",
		Namespace: "default",
	},
	Spec: infrav1.GCPClusterSpec{
		Project: "my-proj",
		Region:  "us-central1",
	},
}

func getFakeGCPMachine() *infrav1.GCPMachine {
	return &infrav1.GCPMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-machine",
			Namespace: "default",
		},
		Spec: infrav1.GCPMachineSpec{
			AdditionalLabels: map[string]string{
				"foo": "bar",
			},
			ResourceManagerTags: []infrav1.ResourceManagerTag{},
		},
	}
}

var fakeGCPMachine = getFakeGCPMachine()

func TestService_createOrGetInstance(t *testing.T) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(fakeBootstrapSecret).
		Build()

	clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Client:        fakec,
		Machine:       fakeMachine,
		GCPMachine:    fakeGCPMachine,
		ClusterGetter: clusterScope,
	})
	if err != nil {
		t.Fatal(err)
	}

	clusterScopeWithoutFailureDomain, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPClusterWithOutFailureDomain,
		GCPServices: scope.GCPServices{
			Compute: &compute.Service{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	machineScopeWithoutFailureDomain, err := scope.NewMachineScope(scope.MachineScopeParams{
		Client:        fakec,
		Machine:       fakeMachineWithOutFailureDomain,
		GCPMachine:    fakeGCPMachine,
		ClusterGetter: clusterScopeWithoutFailureDomain,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		scope        func() Scope
		mockInstance *cloud.MockInstances
		want         *compute.Instance
		wantErr      bool
	}{
		{
			name:  "instance already exist (should return existing instance)",
			scope: func() Scope { return machineScope },
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects: map[meta.Key]*cloud.MockInstancesObj{
					{Name: "my-machine", Zone: "us-central1-c"}: {Obj: &compute.Instance{
						Name: "my-machine",
					}},
				},
			},
			want: &compute.Instance{
				Name: "my-machine",
			},
		},
		{
			name:  "error getting instance with non 404 error code (should return an error)",
			scope: func() Scope { return machineScope },
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
				GetHook: func(_ context.Context, _ *meta.Key, _ *cloud.MockInstances, _ ...cloud.Option) (bool, *compute.Instance, error) {
					return true, &compute.Instance{}, &googleapi.Error{Code: http.StatusBadRequest}
				},
			},
			wantErr: true,
		},
		{
			name:  "instance does not exist (should create instance)",
			scope: func() Scope { return machineScope },
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) and ipForwarding disabled",
			scope: func() Scope {
				ipForwardingDisabled := infrav1.IPForwardingDisabled
				machineScope.GCPMachine = getFakeGCPMachine()
				machineScope.GCPMachine.Spec.IPForwarding = &ipForwardingDisabled
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: false,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) and SecureBoot enabled",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				machineScope.GCPMachine.Spec.ShieldedInstanceConfig = &infrav1.GCPShieldedInstanceConfig{
					SecureBoot: infrav1.SecureBootPolicyEnabled,
				}
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:                   "my-machine",
				CanIpForward:           true,
				ShieldedInstanceConfig: &compute.ShieldedInstanceConfig{EnableSecureBoot: true, EnableVtpm: true, EnableIntegrityMonitoring: true},
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with confidential compute enabled and TERMINATE OnHostMaintenance",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				confidentialComputePolicyEnabled := infrav1.ConfidentialComputePolicyEnabled
				machineScope.GCPMachine.Spec.ConfidentialCompute = &confidentialComputePolicyEnabled
				hostMaintenancePolicyTerminate := infrav1.HostMaintenancePolicyTerminate
				machineScope.GCPMachine.Spec.OnHostMaintenance = &hostMaintenancePolicyTerminate
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				ConfidentialInstanceConfig: &compute.ConfidentialInstanceConfig{
					EnableConfidentialCompute: true,
				},
				Scheduling: &compute.Scheduling{
					OnHostMaintenance: strings.ToUpper(string(infrav1.HostMaintenancePolicyTerminate)),
				},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with confidential compute AMDEncryptedVirtualization",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				hostMaintenancePolicyTerminate := infrav1.HostMaintenancePolicyTerminate
				machineScope.GCPMachine.Spec.OnHostMaintenance = &hostMaintenancePolicyTerminate
				confidentialComputeSEV := infrav1.ConfidentialComputePolicySEV
				machineScope.GCPMachine.Spec.ConfidentialCompute = &confidentialComputeSEV
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				ConfidentialInstanceConfig: &compute.ConfidentialInstanceConfig{
					EnableConfidentialCompute: true,
					ConfidentialInstanceType:  "SEV",
				},
				Scheduling: &compute.Scheduling{
					OnHostMaintenance: strings.ToUpper(string(infrav1.HostMaintenancePolicyTerminate)),
				},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with confidential compute AMDEncryptedVirtualizationNestedPaging",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				hostMaintenancePolicyTerminate := infrav1.HostMaintenancePolicyTerminate
				machineScope.GCPMachine.Spec.OnHostMaintenance = &hostMaintenancePolicyTerminate
				confidentialComputeSEVSNP := infrav1.ConfidentialComputePolicySEVSNP
				machineScope.GCPMachine.Spec.ConfidentialCompute = &confidentialComputeSEVSNP
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				ConfidentialInstanceConfig: &compute.ConfidentialInstanceConfig{
					EnableConfidentialCompute: true,
					ConfidentialInstanceType:  "SEV_SNP",
				},
				Scheduling: &compute.Scheduling{
					OnHostMaintenance: strings.ToUpper(string(infrav1.HostMaintenancePolicyTerminate)),
				},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with MIGRATE OnHostMaintenance",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				hostMaintenancePolicyTerminate := infrav1.HostMaintenancePolicyMigrate
				machineScope.GCPMachine.Spec.OnHostMaintenance = &hostMaintenancePolicyTerminate
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink: "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{
					OnHostMaintenance: strings.ToUpper(string(infrav1.HostMaintenancePolicyMigrate)),
				},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name:  "FailureDomain not given (should pick up a failure domain from the cluster)",
			scope: func() Scope { return machineScopeWithoutFailureDomain },
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			wantErr: false,
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-a/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-a/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-a/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-a",
			},
		},
		{
			name: "instance does not exist (should create instance) with Customer-Managed boot DiskEncryption",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				diskEncryption := infrav1.CustomerEncryptionKey{
					KeyType: infrav1.CustomerManagedKey,
					ManagedKey: &infrav1.ManagedKey{
						KMSKeyName: "projects/my-project/locations/us-central1/keyRings/us-central1/cryptoKeys/some-key",
					},
				}
				machineScope.GCPMachine.Spec.RootDiskEncryptionKey = &diskEncryption
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
						DiskEncryptionKey: &compute.CustomerEncryptionKey{
							KmsKeyName: "projects/my-project/locations/us-central1/keyRings/us-central1/cryptoKeys/some-key",
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with Customer-Supplied Raw DiskEncryption",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				diskEncryption := infrav1.CustomerEncryptionKey{
					KeyType: infrav1.CustomerSuppliedKey,
					SuppliedKey: &infrav1.SuppliedKey{
						RawKey: []byte("SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0="),
					},
				}
				machineScope.GCPMachine.Spec.RootDiskEncryptionKey = &diskEncryption
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
						DiskEncryptionKey: &compute.CustomerEncryptionKey{
							RawKey: "SGVsbG8gZnJvbSBHb29nbGUgQ2xvdWQgUGxhdGZvcm0=",
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
		{
			name: "instance does not exist (should create instance) with Customer-Supplied RSA DiskEncryption",
			scope: func() Scope {
				machineScope.GCPMachine = getFakeGCPMachine()
				diskEncryption := infrav1.CustomerEncryptionKey{
					KeyType: infrav1.CustomerSuppliedKey,
					SuppliedKey: &infrav1.SuppliedKey{
						RSAEncryptedKey: []byte("ieCx/NcW06PcT7Ep1X6LUTc/hLvUDYyzSZPPVCVPTVEohpeHASqC8uw5TzyO9U+Fka9JFHiz0mBibXUInrC/jEk014kCK/NPjYgEMOyssZ4ZINPKxlUh2zn1bV+MCaTICrdmuSBTWlUUiFoDiD6PYznLwh8ZNdaheCeZ8ewEXgFQ8V+sDroLaN3Xs3MDTXQEMMoNUXMCZEIpg9Vtp9x2oe=="),
					},
				}
				machineScope.GCPMachine.Spec.RootDiskEncryptionKey = &diskEncryption
				return machineScope
			},
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: true,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:            "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage:         "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
							ResourceManagerTags: map[string]string{},
							Labels: map[string]string{
								"foo": "bar",
							},
						},
						DiskEncryptionKey: &compute.CustomerEncryptionKey{
							RsaEncryptedKey: "ieCx/NcW06PcT7Ep1X6LUTc/hLvUDYyzSZPPVCVPTVEohpeHASqC8uw5TzyO9U+Fka9JFHiz0mBibXUInrC/jEk014kCK/NPjYgEMOyssZ4ZINPKxlUh2zn1bV+MCaTICrdmuSBTWlUUiFoDiD6PYznLwh8ZNdaheCeZ8ewEXgFQ8V+sDroLaN3Xs3MDTXQEMMoNUXMCZEIpg9Vtp9x2oe==",
						},
					},
				},
				Labels: map[string]string{
					"capg-role":               "node",
					"capg-cluster-my-cluster": "owned",
					"foo":                     "bar",
				},
				MachineType: "zones/us-central1-c/machineTypes",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   "user-data",
							Value: ptr.To[string]("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
				},
				Params: &compute.InstanceParams{
					ResourceManagerTags: map[string]string{},
				},
				SelfLink:   "https://www.googleapis.com/compute/v1/projects/proj-id/zones/us-central1-c/instances/my-machine",
				Scheduling: &compute.Scheduling{},
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email:  "default",
						Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
					},
				},
				Tags: &compute.Tags{
					Items: []string{
						"my-cluster-node",
						"my-cluster",
					},
				},
				Zone: "us-central1-c",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := New(tt.scope())
			s.instances = tt.mockInstance
			got, err := s.createOrGetInstance(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.createOrGetInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("Service.createOrGetInstance() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
