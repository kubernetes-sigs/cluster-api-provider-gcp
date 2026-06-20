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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/utils/strings/slices"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

// Confidential VM Technology support depends on the configured machine types.
// reference: https://cloud.google.com/compute/confidential-vm/docs/os-and-machine-type#machine-type
var (
	confidentialMachineSeriesSupportingSev    = []string{"n2d", "c2d", "c3d"}
	confidentialMachineSeriesSupportingSevsnp = []string{"n2d"}
	confidentialMachineSeriesSupportingTdx    = []string{"c3"}
)

// ValidateConfidentialCompute validates that the confidential computing configuration is valid.
func ValidateConfidentialCompute(confidentialCompute *infrav1.ConfidentialComputePolicy, onHostMaintenance *infrav1.HostMaintenancePolicy, instanceType string) error {
	if confidentialCompute != nil && *confidentialCompute != infrav1.ConfidentialComputePolicyDisabled {
		if onHostMaintenance == nil || *onHostMaintenance == infrav1.HostMaintenancePolicyMigrate {
			return fmt.Errorf("ConfidentialCompute require OnHostMaintenance to be set to %s, the current value is: %s", infrav1.HostMaintenancePolicyTerminate, infrav1.HostMaintenancePolicyMigrate)
		}

		machineSeries := strings.Split(instanceType, "-")[0]
		switch *confidentialCompute {
		case infrav1.ConfidentialComputePolicyEnabled, infrav1.ConfidentialComputePolicySEV:
			if !slices.Contains(confidentialMachineSeriesSupportingSev, machineSeries) {
				return fmt.Errorf("ConfidentialCompute %s requires any of the following machine series: %s. %s was found instead", *confidentialCompute, strings.Join(confidentialMachineSeriesSupportingSev, ", "), instanceType)
			}
		case infrav1.ConfidentialComputePolicySEVSNP:
			if !slices.Contains(confidentialMachineSeriesSupportingSevsnp, machineSeries) {
				return fmt.Errorf("ConfidentialCompute %s requires any of the following machine series: %s. %s was found instead", *confidentialCompute, strings.Join(confidentialMachineSeriesSupportingSevsnp, ", "), instanceType)
			}
		case infrav1.ConfidentialComputePolicyTDX:
			if !slices.Contains(confidentialMachineSeriesSupportingTdx, machineSeries) {
				return fmt.Errorf("ConfidentialCompute %s requires any of the following machine series: %s. %s was found instead", *confidentialCompute, strings.Join(confidentialMachineSeriesSupportingTdx, ", "), instanceType)
			}
		default:
			return fmt.Errorf("invalid ConfidentialCompute %s", *confidentialCompute)
		}
	}
	return nil
}

func checkKeyType(key *infrav1.CustomerEncryptionKey) error {
	switch key.KeyType {
	case infrav1.CustomerManagedKey:
		if key.ManagedKey == nil || key.SuppliedKey != nil {
			return errors.New("CustomerEncryptionKey KeyType of Managed requires only ManagedKey to be set")
		}
	case infrav1.CustomerSuppliedKey:
		if key.SuppliedKey == nil || key.ManagedKey != nil {
			return errors.New("CustomerEncryptionKey KeyType of Supplied requires only SuppliedKey to be set")
		}
		if len(key.SuppliedKey.RawKey) > 0 && len(key.SuppliedKey.RSAEncryptedKey) > 0 {
			return errors.New("CustomerEncryptionKey KeyType of Supplied requires either RawKey or RSAEncryptedKey to be set, not both")
		}
	default:
		return fmt.Errorf("invalid value for CustomerEncryptionKey KeyType %s", key.KeyType)
	}
	return nil
}

// ValidateCustomerEncryptionKey validates that the customer encryption key configuration is valid.
func ValidateCustomerEncryptionKey(rootDiskEncryptionKey *infrav1.CustomerEncryptionKey, additionalDisks []infrav1.AttachedDiskSpec) error {
	if rootDiskEncryptionKey != nil {
		if err := checkKeyType(rootDiskEncryptionKey); err != nil {
			return err
		}
	}

	for _, disk := range additionalDisks {
		if disk.EncryptionKey != nil {
			if err := checkKeyType(disk.EncryptionKey); err != nil {
				return err
			}
		}
	}
	return nil
}
