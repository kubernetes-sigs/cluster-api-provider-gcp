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

package shared

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestCalculateReplicaDivisor(t *testing.T) {
	tests := []struct {
		name          string
		nodeLocations []string
		regional      bool
		want          int32
	}{
		{
			name:          "nodeLocation set",
			nodeLocations: []string{"a", "b", "c", "d", "e"},
			regional:      false,
			want:          5,
		},
		{
			name:          "nodeLocations not set but regional",
			nodeLocations: []string{},
			regional:      true,
			want:          3,
		},
		{
			name:          "nodeLocations not set and zonal",
			nodeLocations: []string{},
			regional:      false,
			want:          1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateReplicaDivisor(tt.nodeLocations, tt.regional); got != tt.want {
				t.Errorf("CalculateReplicaDivisor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createManagedMachinePool(nodeLocations []string, name string) *infrav1exp.GCPManagedMachinePool {
	return &infrav1exp.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: infrav1exp.GCPManagedMachinePoolSpec{
			GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
				NodeLocations: nodeLocations,
			},
		},
	}
}

func createMachinePool(replicas int32, name string) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mp-",
			Namespace:    "default",
		},
		Spec: clusterv1.MachinePoolSpec{
			Replicas: &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Name: name,
					},
				},
			},
		},
	}
}

func TestManagedMachinePoolPreflightCheck(t *testing.T) {
	tests := []struct {
		name          string
		nodeLocations []string
		mmpName       string
		mpName        string
		replicas      int32
		regional      bool
		wantErr       bool
		matchSentinel bool
	}{
		{
			name:          "replicas not a multiple of the divisor fails",
			nodeLocations: []string{"us-central1-a", "us-central1-a"},
			mmpName:       "my-machine-pool",
			mpName:        "my-machine-pool",
			replicas:      5,
			regional:      true,
			wantErr:       true,
			matchSentinel: true,
		},
		{
			name:          "replicas not a multiple of the divisor fails when regional (3)",
			nodeLocations: []string{},
			mmpName:       "my-machine-pool",
			mpName:        "my-machine-pool",
			replicas:      5,
			regional:      true,
			wantErr:       true,
			matchSentinel: true,
		},
		{
			name:          "any count passes for a zonal cluster",
			nodeLocations: []string{},
			mmpName:       "my-machine-pool",
			mpName:        "my-machine-pool",
			replicas:      1298,
			regional:      false,
			wantErr:       false,
		},
		{
			name:          "name mismatch is not the sentinel error",
			nodeLocations: []string{},
			mmpName:       "my-machine-pool",
			mpName:        "this-name-does-not-match",
			replicas:      3,
			regional:      false,
			wantErr:       true,
			matchSentinel: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ManagedMachinePoolPreflightCheck(createManagedMachinePool(tt.nodeLocations, tt.mmpName), createMachinePool(tt.replicas, tt.mpName), tt.regional)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.matchSentinel {
					g.Expect(errors.Is(err, ErrInvalidReplicas)).To(BeTrue())
				} else {
					g.Expect(errors.Is(err, ErrInvalidReplicas)).To(BeFalse())
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
