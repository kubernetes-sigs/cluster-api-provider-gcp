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
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
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
			DataSecretName: pointer.String("my-cluster-bootstrap"),
		},
		FailureDomain: pointer.String("us-central1-c"),
		Version:       pointer.String("v1.19.11"),
	},
}

var fakeMachineWithOutFailureDomain = &clusterv1.Machine{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-machine",
		Namespace: "default",
	},
	Spec: clusterv1.MachineSpec{
		Bootstrap: clusterv1.Bootstrap{
			DataSecretName: pointer.String("my-cluster-bootstrap"),
		},
		Version: pointer.String("v1.19.11"),
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

var fakeGCPMachine = &infrav1.GCPMachine{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-machine",
		Namespace: "default",
	},
	Spec: infrav1.GCPMachineSpec{
		AdditionalLabels: map[string]string{
			"foo": "bar",
		},
	},
}

func TestService_createOrGetInstance(t *testing.T) {
	fakec := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(fakeBootstrapSecret).
		Build()

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPCluster,
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

	clusterScopeWithoutFailureDomain, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     fakec,
		Cluster:    fakeCluster,
		GCPCluster: fakeGCPClusterWithOutFailureDomain,
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
				GetHook: func(ctx context.Context, key *meta.Key, m *cloud.MockInstances) (bool, *compute.Instance, error) {
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
							DiskType:    "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage: "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
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
							Value: pointer.String("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
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
							DiskType:    "zones/us-central1-c/diskTypes/pd-standard",
							SourceImage: "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
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
							Value: pointer.String("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
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
			name:  "FailureDomain not given (should pick up a failure domain from the cluster)",
			scope: func() Scope { return machineScopeWithoutFailureDomain },
			mockInstance: &cloud.MockInstances{
				ProjectRouter: &cloud.SingleProjectRouter{ID: "proj-id"},
				Objects:       map[meta.Key]*cloud.MockInstancesObj{},
			},
			wantErr: false,
			want: &compute.Instance{
				Name:         "my-machine",
				CanIpForward: false,
				Disks: []*compute.AttachedDisk{
					{
						AutoDelete: true,
						Boot:       true,
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskType:    "zones/us-central1-a/diskTypes/pd-standard",
							SourceImage: "projects/my-proj/global/images/family/capi-ubuntu-1804-k8s-v1-19",
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
							Value: pointer.String("Zm9vCg=="),
						},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: "projects/my-proj/global/networks/default",
					},
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
