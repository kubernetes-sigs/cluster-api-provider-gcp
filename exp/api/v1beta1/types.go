/*
Copyright 2022 The Kubernetes Authors.

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

package v1beta1

import (
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/pointer"
)

// TaintEffect is the effect for a Kubernetes taint.
type TaintEffect string

// Taint represents a Kubernetes taint.
type Taint struct {
	// Effect specifies the effect for the taint.
	// +kubebuilder:validation:Enum=NoSchedule;NoExecute;PreferNoSchedule
	Effect TaintEffect `json:"effect"`
	// Key is the key of the taint
	Key string `json:"key"`
	// Value is the value of the taint
	Value string `json:"value"`
}

// Taints is an array of Taints.
type Taints []Taint

func convertToSdkTaintEffect(effect TaintEffect) containerpb.NodeTaint_Effect {
	switch effect {
	case "NoSchedule":
		return containerpb.NodeTaint_NO_SCHEDULE
	case "NoExecute":
		return containerpb.NodeTaint_NO_EXECUTE
	case "PreferNoSchedule":
		return containerpb.NodeTaint_PREFER_NO_SCHEDULE
	default:
		return containerpb.NodeTaint_EFFECT_UNSPECIFIED
	}
}

// ConvertToSdkTaint converts taints to format that is used by GCP SDK.
func ConvertToSdkTaint(taints Taints) []*containerpb.NodeTaint {
	if taints == nil {
		return nil
	}
	res := []*containerpb.NodeTaint{}
	for _, taint := range taints {
		res = append(res, &containerpb.NodeTaint{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: convertToSdkTaintEffect(taint.Effect),
		})
	}
	return res
}

// ConvertToSdkReleaseChannel converts the CAPG ReleaseChannel enum to a containerpb ReleaseChannel_Channel.
func ConvertToSdkReleaseChannel(channel *ReleaseChannel) containerpb.ReleaseChannel_Channel {
	if channel == nil {
		return containerpb.ReleaseChannel_UNSPECIFIED
	}
	switch *channel {
	case Rapid:
		return containerpb.ReleaseChannel_RAPID
	case Regular:
		return containerpb.ReleaseChannel_REGULAR
	case Stable:
		return containerpb.ReleaseChannel_STABLE
	default:
		return containerpb.ReleaseChannel_UNSPECIFIED
	}
}

// ConvertToSdkAddonsConfig converts the CAPG AddonsConfig to a containerpb AddonsConfig.
func ConvertToSdkAddonsConfig(config *AddonsConfig) *containerpb.AddonsConfig {
	addonsConfig := &containerpb.AddonsConfig{
		// Set to false by default
		DnsCacheConfig: &containerpb.DnsCacheConfig{
			Enabled: false,
		},
		// Set to true by default
		GcePersistentDiskCsiDriverConfig: &containerpb.GcePersistentDiskCsiDriverConfig{
			Enabled: true,
		},
	}

	// If not specified return defaults
	if config == nil {
		return addonsConfig
	}

	if config.DNSCacheConfig != nil {
		addonsConfig.DnsCacheConfig = &containerpb.DnsCacheConfig{
			Enabled: config.DNSCacheConfig.Enabled,
		}
	}

	if config.GcePersistentDiskCsiDriverConfig != nil {
		addonsConfig.GcePersistentDiskCsiDriverConfig = &containerpb.GcePersistentDiskCsiDriverConfig{
			Enabled: config.GcePersistentDiskCsiDriverConfig.Enabled,
		}
	}

	return addonsConfig
}

// ConvertToSdkIPAllocationPolicy converts the CAPG IPAllocationPolicy to a containerpb IPAllocationPolicy.
func ConvertToSdkIPAllocationPolicy(policy *IPAllocationPolicy) *containerpb.IPAllocationPolicy {
	if policy == nil {
		return nil
	}

	return &containerpb.IPAllocationPolicy{
		UseIpAliases:               pointer.BoolDeref(policy.UseIPAliases, false),
		ClusterSecondaryRangeName:  pointer.StringDeref(policy.ClusterSecondaryRangeName, ""),
		ServicesSecondaryRangeName: pointer.StringDeref(policy.ServicesSecondaryRangeName, ""),
		ClusterIpv4CidrBlock:       pointer.StringDeref(policy.ClusterIpv4CidrBlock, ""),
		ServicesIpv4CidrBlock:      pointer.StringDeref(policy.ServicesIpv4CidrBlock, ""),
	}
}

// ConvertToSdkLoggingConfig converts the CAPG LoggingConfig to a containerpb LoggingConfig.
func ConvertToSdkLoggingConfig(config *LoggingConfig) *containerpb.LoggingConfig {
	// If not specified, return the empty set that disables all components
	if config == nil {
		return &containerpb.LoggingConfig{
			ComponentConfig: &containerpb.LoggingComponentConfig{
				EnableComponents: []containerpb.LoggingComponentConfig_Component{},
			},
		}
	}

	enableComponents := []containerpb.LoggingComponentConfig_Component{}
	for _, c := range config.EnableComponents {
		var loggingComponent containerpb.LoggingComponentConfig_Component
		switch c {
		case SystemComponents:
			loggingComponent = containerpb.LoggingComponentConfig_SYSTEM_COMPONENTS
		case Workloads:
			loggingComponent = containerpb.LoggingComponentConfig_WORKLOADS
		case APIServer:
			loggingComponent = containerpb.LoggingComponentConfig_APISERVER
		case Scheduler:
			loggingComponent = containerpb.LoggingComponentConfig_SCHEDULER
		case ControllerManager:
			loggingComponent = containerpb.LoggingComponentConfig_CONTROLLER_MANAGER
		default:
			loggingComponent = containerpb.LoggingComponentConfig_COMPONENT_UNSPECIFIED
		}
		enableComponents = append(enableComponents, loggingComponent)
	}

	return &containerpb.LoggingConfig{
		ComponentConfig: &containerpb.LoggingComponentConfig{
			EnableComponents: enableComponents,
		},
	}
}

// ConvertToSdkMaintenancePolicy converts the CAPG MaintenancePolicy to a containerpb MaintenancePolicy.
func ConvertToSdkMaintenancePolicy(policy *MaintenancePolicy) (*containerpb.MaintenancePolicy, error) {
	if policy == nil {
		return nil, nil
	}

	maintenancePolicy := &containerpb.MaintenancePolicy{}

	var exclusions map[string]*containerpb.TimeWindow
	if policy.MaintenanceExclusions != nil {
		exclusions = map[string]*containerpb.TimeWindow{}
		for k, v := range policy.MaintenanceExclusions {
			tw, err := ConvertToSdkTimeWindow(v)
			if err != nil {
				return nil, err
			}
			exclusions[k] = tw
		}
	}

	if policy.DailyMaintenanceWindow != nil {
		maintenancePolicy.Window = &containerpb.MaintenanceWindow{
			Policy: &containerpb.MaintenanceWindow_DailyMaintenanceWindow{
				DailyMaintenanceWindow: &containerpb.DailyMaintenanceWindow{
					StartTime: policy.DailyMaintenanceWindow.StartTime,
				},
			},
			MaintenanceExclusions: exclusions,
		}
	}

	if policy.RecurringMaintenanceWindow != nil {
		tw, err := ConvertToSdkTimeWindow(policy.RecurringMaintenanceWindow.Window)
		if err != nil {
			return nil, err
		}
		maintenancePolicy.Window = &containerpb.MaintenanceWindow{
			Policy: &containerpb.MaintenanceWindow_RecurringWindow{
				RecurringWindow: &containerpb.RecurringTimeWindow{
					Window:     tw,
					Recurrence: policy.RecurringMaintenanceWindow.Recurrence,
				},
			},
			MaintenanceExclusions: exclusions,
		}
	}

	return maintenancePolicy, nil
}

// ConvertToSdkTimeWindow converts the CAPG TimeWindow to a containerpb TimeWindow. It will return an error
// if the start and end times fail to parse into valid RFC3339 timestamps.
func ConvertToSdkTimeWindow(window *TimeWindow) (*containerpb.TimeWindow, error) {
	if window == nil {
		return nil, nil
	}

	tw := &containerpb.TimeWindow{}

	st, err := time.Parse(time.RFC3339, window.StartTime)
	if err != nil {
		return nil, err
	}
	tw.StartTime = timestamppb.New(st)

	et, err := time.Parse(time.RFC3339, window.EndTime)
	if err != nil {
		return nil, err
	}
	tw.EndTime = timestamppb.New(et)

	if window.MaintenanceExclusionOption != nil {
		twOptions := &containerpb.TimeWindow_MaintenanceExclusionOptions{}
		twOptions.MaintenanceExclusionOptions = &containerpb.MaintenanceExclusionOptions{}
		switch *window.MaintenanceExclusionOption {
		case NoUpgrades:
			twOptions.MaintenanceExclusionOptions.Scope = containerpb.MaintenanceExclusionOptions_NO_UPGRADES
		case NoMinorUpgrades:
			twOptions.MaintenanceExclusionOptions.Scope = containerpb.MaintenanceExclusionOptions_NO_MINOR_UPGRADES
		case NoMinorOrNodeUpgrades:
			twOptions.MaintenanceExclusionOptions.Scope = containerpb.MaintenanceExclusionOptions_NO_MINOR_OR_NODE_UPGRADES
		default:
			twOptions.MaintenanceExclusionOptions.Scope = containerpb.MaintenanceExclusionOptions_NO_UPGRADES
		}
		tw.Options = twOptions
	}

	return tw, nil
}

// ConvertToSdkMasterAuthorizedNetworksConfig converts the MasterAuthorizedNetworksConfig defined in CRs to the SDK version.
func ConvertToSdkMasterAuthorizedNetworksConfig(config *MasterAuthorizedNetworksConfig) *containerpb.MasterAuthorizedNetworksConfig {
	// if config is nil, it means that the user wants to disable the feature.
	if config == nil {
		return &containerpb.MasterAuthorizedNetworksConfig{
			Enabled:                     false,
			CidrBlocks:                  []*containerpb.MasterAuthorizedNetworksConfig_CidrBlock{},
			GcpPublicCidrsAccessEnabled: new(bool),
		}
	}

	// Convert the CidrBlocks slice.
	cidrBlocks := make([]*containerpb.MasterAuthorizedNetworksConfig_CidrBlock, len(config.CidrBlocks))
	for i, cidrBlock := range config.CidrBlocks {
		cidrBlocks[i] = &containerpb.MasterAuthorizedNetworksConfig_CidrBlock{
			CidrBlock:   cidrBlock.CidrBlock,
			DisplayName: cidrBlock.DisplayName,
		}
	}

	return &containerpb.MasterAuthorizedNetworksConfig{
		Enabled:                     true,
		CidrBlocks:                  cidrBlocks,
		GcpPublicCidrsAccessEnabled: config.GcpPublicCidrsAccessEnabled,
	}
}

// ConvertToSdkNetworkConfig converts the CAPG NetworkConfig to the containerpb NetworkConfig.
func ConvertToSdkNetworkConfig(config *NetworkConfig) *containerpb.NetworkConfig {
	if config == nil {
		return &containerpb.NetworkConfig{}
	}

	networkConfig := &containerpb.NetworkConfig{}
	if config.DatapathProvider != nil {
		switch *config.DatapathProvider {
		case LegacyDatapath:
			networkConfig.DatapathProvider = containerpb.DatapathProvider_LEGACY_DATAPATH
		case AdvancedDatapath:
			networkConfig.DatapathProvider = containerpb.DatapathProvider_ADVANCED_DATAPATH
		default:
			networkConfig.DatapathProvider = containerpb.DatapathProvider_DATAPATH_PROVIDER_UNSPECIFIED
		}
	}

	if config.DNSConfig != nil {
		dnsConfig := &containerpb.DNSConfig{}
		if config.DNSConfig.ClusterDNS != nil {
			switch *config.DNSConfig.ClusterDNS {
			case PlatformDefault:
				dnsConfig.ClusterDns = containerpb.DNSConfig_PLATFORM_DEFAULT
			case CloudDNS:
				dnsConfig.ClusterDns = containerpb.DNSConfig_CLOUD_DNS
			case KubeDNS:
				dnsConfig.ClusterDns = containerpb.DNSConfig_KUBE_DNS
			default:
				dnsConfig.ClusterDns = containerpb.DNSConfig_PROVIDER_UNSPECIFIED
			}
		}

		if config.DNSConfig.ClusterDNSScope != nil {
			switch *config.DNSConfig.ClusterDNSScope {
			case ClusterScope:
				dnsConfig.ClusterDnsScope = containerpb.DNSConfig_CLUSTER_SCOPE
			case VpcScope:
				dnsConfig.ClusterDnsScope = containerpb.DNSConfig_VPC_SCOPE
			default:
				dnsConfig.ClusterDnsScope = containerpb.DNSConfig_DNS_SCOPE_UNSPECIFIED
			}
		}

		if config.DNSConfig.ClusterDNSDomain != nil {
			dnsConfig.ClusterDnsDomain = *config.DNSConfig.ClusterDNSDomain
		}

		networkConfig.DnsConfig = dnsConfig
	}

	return networkConfig
}

// ConvertToSdkPrivateClusterConfig converts the CAPG PrivateClusterConfig to the containerpb PrivateClusterConfig.
func ConvertToSdkPrivateClusterConfig(config *PrivateClusterConfig) *containerpb.PrivateClusterConfig {
	if config == nil {
		// Default value in containerpb.Cluster.PrivateClusterConfig is nil.
		// Return nil for update comparison.
		return nil
	}

	return &containerpb.PrivateClusterConfig{
		EnablePrivateNodes:    config.EnablePrivateNodes,
		EnablePrivateEndpoint: config.EnablePrivateEndpoint,
		MasterIpv4CidrBlock:   pointer.StringDeref(config.MasterIpv4CidrBlock, ""),
		MasterGlobalAccessConfig: &containerpb.PrivateClusterMasterGlobalAccessConfig{
			Enabled: pointer.BoolDeref(config.PrivateClusterMasterGlobalAccessEnabled, false),
		},
		PrivateEndpointSubnetwork: pointer.StringDeref(config.PrivateEndpointSubnetwork, ""),
	}
}

// ConvertToSdkShieldedNodes converts the CAPG ShieldedNodes to a containerpb ShieldedNodes.
func ConvertToSdkShieldedNodes(config *ShieldedNodes) *containerpb.ShieldedNodes {
	if config == nil {
		// ShieldedNodes is enabled by default
		return &containerpb.ShieldedNodes{Enabled: true}
	}

	return &containerpb.ShieldedNodes{Enabled: config.Enabled}
}

// ConvertToSdkWorkloadIdentityConfig converts the CAPG WorkloadIdentityConfig to a containerpb WorkloadIdentityConfig.
func ConvertToSdkWorkloadIdentityConfig(config *WorkloadIdentityConfig) *containerpb.WorkloadIdentityConfig {
	if config == nil {
		// Default value in containerpb.Cluster.WorkloadIdentityConfig is nil.
		// Return nil for update comparison.
		return nil
	}
	return &containerpb.WorkloadIdentityConfig{
		WorkloadPool: config.WorkloadPool,
	}
}
