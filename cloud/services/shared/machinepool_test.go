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
	"testing"

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

func createManagedMachinePool(nodeLocations []string) *infrav1exp.GCPManagedMachinePool {
	return &infrav1exp.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mmp-",
			Namespace:    "default",
		},
		Spec: infrav1exp.GCPManagedMachinePoolSpec{
			GCPManagedMachinePoolClassSpec: infrav1exp.GCPManagedMachinePoolClassSpec{
				NodeLocations: nodeLocations,
			},
		},
	}
}

func createMachinePool(replicas int32) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mp-",
			Namespace:    "default",
		},
		Spec: clusterv1.MachinePoolSpec{
			Replicas: &replicas,
		},
	}
}

func TestManagedMachinePoolPreflightCheck(t *testing.T) {
	tests := []struct {
		name          string
		nodeLocations []string
		replicas      int32
		regional      bool
		wantErr       bool
	}{
		{
			name:          "replicas not a multiple of the divisor fails",
			nodeLocations: []string{"us-central1-a", "us-central1-a"},
			replicas:      5,
			regional:      true,
			wantErr:       true,
		},
		{
			name:          "replicas not a multiple of the divisor fails when regional (3)",
			nodeLocations: []string{},
			replicas:      5,
			regional:      true,
			wantErr:       true,
		},
		{
			name:          "any count passes for a zonal cluster",
			nodeLocations: []string{},
			replicas:      1298,
			regional:      false,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ManagedMachinePoolPreflightCheck(createManagedMachinePool(tt.nodeLocations), createMachinePool(tt.replicas), tt.regional); (err != nil) != tt.wantErr {
				t.Errorf("ManagedMachinePoolPreflightCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
