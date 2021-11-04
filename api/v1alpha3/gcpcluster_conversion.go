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

	v1beta1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

// ConvertTo converts this GCPCluster to the Hub version (v1beta1).
func (src *GCPCluster) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1beta1.GCPCluster)

	if err := Convert_v1alpha3_GCPCluster_To_v1beta1_GCPCluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *GCPCluster) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1beta1.GCPCluster)

	if err := Convert_v1beta1_GCPCluster_To_v1alpha3_GCPCluster(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this GCPClusterList to the Hub version (v1beta1).
func (src *GCPClusterList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1beta1.GCPClusterList)
	return Convert_v1alpha3_GCPClusterList_To_v1beta1_GCPClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *GCPClusterList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1beta1.GCPClusterList)
	return Convert_v1beta1_GCPClusterList_To_v1alpha3_GCPClusterList(src, dst, nil)
}

// Convert_v1alpha3_GCPClusterStatus_To_v1beta1_GCPClusterStatuss converts GCPCluster.Status from v1alpha3 to v1beta1.
func Convert_v1alpha3_GCPClusterStatus_To_v1beta1_GCPClusterStatus(in *GCPClusterStatus, out *v1beta1.GCPClusterStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_GCPClusterStatus_To_v1beta1_GCPClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_GCPClusterSpec_To_v1beta1_GCPClusterSpec.
func Convert_v1alpha3_GCPClusterSpec_To_v1beta1_GCPClusterSpec(in *GCPClusterSpec, out *v1beta1.GCPClusterSpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha3_GCPClusterSpec_To_v1beta1_GCPClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1beta1_GCPClusterSpec_To_v1alpha3_GCPClusterSpec converts from the Hub version (v1beta1) of the GCPClusterSpec to this version.
func Convert_v1beta1_GCPClusterSpec_To_v1alpha3_GCPClusterSpec(in *v1beta1.GCPClusterSpec, out *GCPClusterSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1beta1_GCPClusterSpec_To_v1alpha3_GCPClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1beta1_GCPClusterStatus_To_v1alpha3_GCPClusterStatus.
func Convert_v1beta1_GCPClusterStatus_To_v1alpha3_GCPClusterStatus(in *v1beta1.GCPClusterStatus, out *GCPClusterStatus, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1beta1_GCPClusterStatus_To_v1alpha3_GCPClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec.
func Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec(in *NetworkSpec, out *v1beta1.NetworkSpec, s apiconversion.Scope) error { //nolint
	out.Subnets = make(v1beta1.Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = &v1beta1.SubnetSpec{}
		if err := Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(in.Subnets[i], out.Subnets[i], s); err != nil {
			return err
		}
	}

	return nil
}

// Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec.
func Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec(in *v1beta1.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error { //nolint
	out.Subnets = make(Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = &SubnetSpec{}
		if err := Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(in.Subnets[i], out.Subnets[i], s); err != nil {
			return err
		}
	}

	return nil
}
