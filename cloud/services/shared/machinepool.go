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

package shared

import (
	"errors"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

const (
	// DefaultNumZonesPerRegion is the default number of zones per region.
	DefaultNumZonesPerRegion = 3
)

// ManagedMachinePoolPreflightCheck will perform checks against the machine pool before its created.
func ManagedMachinePoolPreflightCheck(managedPool *infrav1exp.GCPManagedMachinePool, machinePool *clusterv1.MachinePool, regional bool) error {
	if machinePool.Spec.Template.Spec.InfrastructureRef.Name != managedPool.Name {
		return fmt.Errorf("expect machinepool infraref (%s) to match managed machine pool name (%s)", machinePool.Spec.Template.Spec.InfrastructureRef.Name, managedPool.Name)
	}

	divisor := CalculateReplicaDivisor(managedPool.Spec.NodeLocations, regional)

	return ValidateReplicas(*machinePool.Spec.Replicas, divisor)
}

// ValidateReplicas ensures the replicas are a multiple of the zone count (the divisor), because GKE creates the same
// number of nodes in every zone of a node pool.
func ValidateReplicas(replicas, divisor int32) error {
	if divisor <= 0 {
		return fmt.Errorf("divisor must be positive: %d", divisor)
	}
	if replicas%divisor != 0 {
		return fmt.Errorf("replicas (%d) must be a multiple of the zone count (%d)", replicas, divisor)
	}
	return nil
}

// ManagedMachinePoolsPreflightCheck will perform checks against a slice of machine pool before they are created.
func ManagedMachinePoolsPreflightCheck(managedPools []infrav1exp.GCPManagedMachinePool, machinePools []clusterv1.MachinePool, regional bool) error {
	if len(machinePools) != len(managedPools) {
		return errors.New("each machinepool must have a matching gcpmanagedmachinepool")
	}

	for i := range machinePools {
		machinepool := machinePools[i]
		managedPool := managedPools[i]

		if err := ManagedMachinePoolPreflightCheck(&managedPool, &machinepool, regional); err != nil {
			return err
		}
	}

	return nil
}

// CalculateReplicaDivisor works out the necessary divisor to convert from the number of replicas
// set on the machinepool to the number we need to pass to the GKE API. The rule is:
// - If nodeLocations has any value at all, prefer it
// - If not, and we know the cluster is regional, return the default number of zones per region
// - If all else fails return 1
func CalculateReplicaDivisor(nodeLocations []string, regional bool) int32 {
	if len(nodeLocations) != 0 {
		return int32(len(nodeLocations))
	}
	if regional {
		return DefaultNumZonesPerRegion
	}
	return 1
}
