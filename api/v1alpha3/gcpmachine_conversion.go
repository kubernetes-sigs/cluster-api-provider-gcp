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

package v1alpha3

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1alpha4 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha4"
)

// ConvertTo converts this GCPMachine to the Hub version (v1alpha4).
func (src *GCPMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1alpha4.GCPMachine)

	if err := Convert_v1alpha3_GCPMachine_To_v1alpha4_GCPMachine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *GCPMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1alpha4.GCPMachine)
	if err := Convert_v1alpha4_GCPMachine_To_v1alpha3_GCPMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this GCPMachineList to the Hub version (v1alpha4).
func (src *GCPMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1alpha4.GCPMachineList)
	return Convert_v1alpha3_GCPMachineList_To_v1alpha4_GCPMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *GCPMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1alpha4.GCPMachineList)
	return Convert_v1alpha4_GCPMachineList_To_v1alpha3_GCPMachineList(src, dst, nil)
}

func Convert_v1alpha3_GCPMachineSpec_To_v1alpha4_GCPMachineSpec(in *GCPMachineSpec, out *v1alpha4.GCPMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_GCPMachineSpec_To_v1alpha4_GCPMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_GCPMachineSpec_To_v1alpha3_GCPMachineSpec converts from the Hub version (v1alpha4) of the GCPMachineSpec to this version.
func Convert_v1alpha4_GCPMachineSpec_To_v1alpha3_GCPMachineSpec(in *v1alpha4.GCPMachineSpec, out *GCPMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_GCPMachineSpec_To_v1alpha3_GCPMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_GCPMachineStatus_To_v1alpha4_GCPMachineStatus converts this GCPMachineStatus to the Hub version (v1alpha4).
func Convert_v1alpha3_GCPMachineStatus_To_v1alpha4_GCPMachineStatus(in *GCPMachineStatus, out *v1alpha4.GCPMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_GCPMachineStatus_To_v1alpha4_GCPMachineStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_GCPMachineStatus_To_v1alpha3_GCPMachineStatus converts from the Hub version (v1alpha4) of the GCPMachineStatus to this version.
func Convert_v1alpha4_GCPMachineStatus_To_v1alpha3_GCPMachineStatus(in *v1alpha4.GCPMachineStatus, out *GCPMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_GCPMachineStatus_To_v1alpha3_GCPMachineStatus(in, out, s); err != nil {
		return err
	}

	return nil
}
