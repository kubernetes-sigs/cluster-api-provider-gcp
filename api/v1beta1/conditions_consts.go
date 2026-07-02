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

package v1beta1

const (
	// WaitingForClusterInfrastructureReason used when machine is waiting for cluster infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
)

// GCPCluster v1beta2 condition types.
const (
	// GCPClusterReadyCondition reports on the overall readiness of the GCPCluster.
	GCPClusterReadyCondition = "Ready"
	// GCPClusterNetworkReadyCondition reports on the readiness of the cluster network.
	GCPClusterNetworkReadyCondition = "NetworkReady"
	// GCPClusterSubnetsReadyCondition reports on the readiness of the cluster subnets.
	GCPClusterSubnetsReadyCondition = "SubnetsReady"
	// GCPClusterFirewallRulesReadyCondition reports on the readiness of the cluster firewall rules.
	GCPClusterFirewallRulesReadyCondition = "FirewallRulesReady"
	// GCPClusterLoadBalancerReadyCondition reports on the readiness of the cluster load balancer.
	GCPClusterLoadBalancerReadyCondition = "LoadBalancerReady"
)

// GCPCluster v1beta2 reason strings.
const (
	// NetworkReadyReason used when the network is ready.
	NetworkReadyReason = "NetworkReady"
	// NetworkReconciliationFailedReason used when network reconciliation has failed.
	NetworkReconciliationFailedReason = "NetworkReconciliationFailed"
	// SubnetsReadyReason used when the subnets are ready.
	SubnetsReadyReason = "SubnetsReady"
	// SubnetsReconciliationFailedReason used when subnets reconciliation has failed.
	SubnetsReconciliationFailedReason = "SubnetsReconciliationFailed"
	// FirewallRulesReadyReason used when the firewall rules are ready.
	FirewallRulesReadyReason = "FirewallRulesReady"
	// FirewallRulesReconciliationFailedReason used when firewall rules reconciliation has failed.
	FirewallRulesReconciliationFailedReason = "FirewallRulesReconciliationFailed"
	// LoadBalancerReadyReason used when the load balancer is ready.
	LoadBalancerReadyReason = "LoadBalancerReady"
	// LoadBalancerReconciliationFailedReason used when load balancer reconciliation has failed.
	LoadBalancerReconciliationFailedReason = "LoadBalancerReconciliationFailed"
)

// GCPMachine v1beta2 condition types.
const (
	// GCPMachineReadyCondition reports on the overall readiness of the GCPMachine.
	GCPMachineReadyCondition = "Ready"
	// GCPMachineInstanceReadyCondition reports on the readiness of the GCP compute instance.
	GCPMachineInstanceReadyCondition = "InstanceReady"
)

// GCPMachine v1beta2 reason strings.
const (
	// InstanceReadyReason used when the instance is ready.
	InstanceReadyReason = "InstanceReady"
	// InstanceNotReadyReason used when the instance exists but is not yet in a running state.
	InstanceNotReadyReason = "InstanceNotReady"
	// InstanceReconciliationFailedReason used when instance reconciliation has failed.
	InstanceReconciliationFailedReason = "InstanceReconciliationFailed"
)
