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

package clusters

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/pointer"
)

// compareAddonsConfig compares if two AddonsConfig are equal.
func compareAddonsConfig(a, b *containerpb.AddonsConfig) bool {
	if a == nil && b == nil {
		return true
	}

	// Normalize nil values for specific field comparisons
	if a == nil {
		a = &containerpb.AddonsConfig{}
	}

	if b == nil {
		b = &containerpb.AddonsConfig{}
	}

	// Nil values are treated as false
	if (a.DnsCacheConfig == nil && b.DnsCacheConfig != nil && b.DnsCacheConfig.Enabled) ||
		(a.DnsCacheConfig != nil && a.DnsCacheConfig.Enabled && b.DnsCacheConfig == nil) ||
		(a.DnsCacheConfig != nil && b.DnsCacheConfig != nil && (a.DnsCacheConfig.Enabled != b.DnsCacheConfig.Enabled)) {
		return false
	}

	// Nil values are treated as false
	if (a.GcePersistentDiskCsiDriverConfig == nil && b.GcePersistentDiskCsiDriverConfig != nil && b.GcePersistentDiskCsiDriverConfig.Enabled) ||
		(a.GcePersistentDiskCsiDriverConfig != nil && a.GcePersistentDiskCsiDriverConfig.Enabled && b.GcePersistentDiskCsiDriverConfig == nil) ||
		(a.GcePersistentDiskCsiDriverConfig != nil && b.GcePersistentDiskCsiDriverConfig != nil && (a.GcePersistentDiskCsiDriverConfig.Enabled != b.GcePersistentDiskCsiDriverConfig.Enabled)) {
		return false
	}

	return true
}

// compareLoggingConfig compares if two LoggingConfig are equal.
func compareLoggingConfig(a, b *containerpb.LoggingConfig) bool {
	if a == nil && b == nil {
		return true
	}

	// Normalize nil values for specific field comparisons
	if a == nil {
		a = &containerpb.LoggingConfig{}
	}

	if b == nil {
		b = &containerpb.LoggingConfig{}
	}

	if a.ComponentConfig == nil && b.ComponentConfig == nil {
		return true
	}

	if a.ComponentConfig == nil {
		a.ComponentConfig = &containerpb.LoggingComponentConfig{}
	}

	if b.ComponentConfig == nil {
		b.ComponentConfig = &containerpb.LoggingComponentConfig{}
	}

	// Nil values are treated as empty list
	if len(a.ComponentConfig.EnableComponents) == 0 && len(b.ComponentConfig.EnableComponents) == 0 {
		return true
	}

	if !cmp.Equal(a.ComponentConfig.EnableComponents, b.ComponentConfig.EnableComponents) {
		return false
	}

	return true
}

// compareMaintenancePolicy compares if two MaintenancePolicy are equal.
func compareMaintenancePolicy(a, b *containerpb.MaintenancePolicy) bool {
	if a == nil && b == nil {
		return true
	}

	// Normalize nil values for specific field comparisons
	if a == nil {
		a = &containerpb.MaintenancePolicy{}
	}

	if b == nil {
		b = &containerpb.MaintenancePolicy{}
	}

	if !cmp.Equal(a.Window.GetDailyMaintenanceWindow().GetStartTime(), b.Window.GetDailyMaintenanceWindow().GetStartTime()) {
		return false
	}

	timeWindowComparator := cmp.Comparer(compareTimeWindow)

	if !cmp.Equal(a.Window.GetRecurringWindow(), b.Window.GetRecurringWindow(), timeWindowComparator,
		cmpopts.IgnoreUnexported(containerpb.RecurringTimeWindow{})) {
		return false
	}

	if !cmp.Equal(a.Window.GetMaintenanceExclusions(), b.Window.GetMaintenanceExclusions(), timeWindowComparator) {
		return false
	}

	return true
}

// compareTimeWindow compares if two TimeWindows are equal.
func compareTimeWindow(a, b *containerpb.TimeWindow) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if !cmp.Equal(a.GetMaintenanceExclusionOptions(), b.GetMaintenanceExclusionOptions(),
		cmpopts.IgnoreUnexported(containerpb.MaintenanceExclusionOptions{})) {
		return false
	}

	if (a.StartTime != nil && b.StartTime == nil) || (a.StartTime == nil && b.StartTime != nil) ||
		(a.StartTime != nil && b.StartTime != nil && !a.StartTime.AsTime().Equal(b.StartTime.AsTime())) {
		return false
	}

	if (a.EndTime != nil && b.EndTime == nil) || (a.EndTime == nil && b.EndTime != nil) ||
		(a.EndTime != nil && b.EndTime != nil && !a.EndTime.AsTime().Equal(b.EndTime.AsTime())) {
		return false
	}

	return true
}

// compareMasterAuthorizedNetworksConfig compares if two MasterAuthorizedNetworksConfig are equal.
func compareMasterAuthorizedNetworksConfig(a, b *containerpb.MasterAuthorizedNetworksConfig) bool {
	if a == nil && b == nil {
		return true
	}

	// Normalize nil values for specific field comparisons
	if a == nil {
		a = &containerpb.MasterAuthorizedNetworksConfig{}
	}

	if b == nil {
		b = &containerpb.MasterAuthorizedNetworksConfig{}
	}

	if a.Enabled != b.Enabled {
		return false
	}

	// Nil values are treated as false
	if pointer.BoolDeref(a.GcpPublicCidrsAccessEnabled, false) != pointer.BoolDeref(b.GcpPublicCidrsAccessEnabled, false) {
		return false
	}

	// Nil values are treated as empty list
	if (a.CidrBlocks == nil && b.CidrBlocks != nil && len(b.CidrBlocks) > 0) ||
		(a.CidrBlocks != nil && len(a.CidrBlocks) > 0 && b.CidrBlocks == nil) {
		return false
	}

	if !cmp.Equal(a.CidrBlocks, b.CidrBlocks, cmpopts.IgnoreUnexported(containerpb.MasterAuthorizedNetworksConfig_CidrBlock{})) {
		return false
	}

	return true
}

// compareMasterGlobalAccessConfig compares if two PrivateClusterMasterGlobalAccessConfig are equal.
func compareMasterGlobalAccessConfig(a, b *containerpb.PrivateClusterMasterGlobalAccessConfig) bool {
	if a == nil && b == nil {
		return true
	}

	// Nil values are treated as false
	if a == nil && b != nil && b.Enabled ||
		a != nil && a.Enabled && b == nil ||
		a != nil && b != nil && a.Enabled != b.Enabled {
		return false
	}

	return true
}

// compareWorkloadIdentityConfig compares if two WorkloadIdentityConfig are equal.
func compareWorkloadIdentityConfig(a, b *containerpb.WorkloadIdentityConfig) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil {
		a = &containerpb.WorkloadIdentityConfig{}
	}

	if b == nil {
		b = &containerpb.WorkloadIdentityConfig{}
	}

	return cmp.Equal(a, b, cmpopts.IgnoreUnexported(containerpb.WorkloadIdentityConfig{}))
}
