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

package loadbalancers

import (
	"fmt"

	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

func hasRegionalExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
	return lbType == infrav1.RegionalExternal ||
		lbType == infrav1.RegionalInternalExternal
}

// ensureRegionalProxyOnlySubnet checks that the cluster's declared subnets
// include at least one proxy-only subnet (Purpose=REGIONAL_MANAGED_PROXY) in
// the load balancer's region — a hard prerequisite for a Regional External
// Proxy LB. Without it, GCP rejects the forwarding rule with an opaque error;
// failing fast here produces an actionable message pointing at the CAPG spec.
func (s *Service) ensureRegionalProxyOnlySubnet() error {
	region := s.scope.Region()
	for _, subnet := range s.scope.SubnetSpecs() {
		if subnet.Purpose != subnetPurposeRegionalManagedProxy {
			continue
		}
		// SubnetSpec.Region is optional; an empty value defers to scope.Region().
		if subnet.Region == "" || subnet.Region == region {
			return nil
		}
	}
	return fmt.Errorf(
		"regional external load balancer requires a proxy-only subnet with purpose %q in region %q; "+
			"add one to spec.network.subnets (see https://cloud.google.com/load-balancing/docs/proxy-only-subnets)",
		subnetPurposeRegionalManagedProxy, region,
	)
}

func hasInternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
	return lbType == infrav1.Internal ||
		lbType == infrav1.InternalExternal ||
		lbType == infrav1.RegionalInternalExternal
}

func getInternalLoadBalancerName(lbSpec infrav1.LoadBalancerSpec) string {
	if lbSpec.InternalLoadBalancer != nil {
		return ptr.Deref(lbSpec.InternalLoadBalancer.Name, infrav1.InternalRoleTagValue)
	}
	return infrav1.InternalRoleTagValue
}

func getExternalLoadBalancerName(lbSpec infrav1.LoadBalancerSpec) string {
	if lbSpec.ExternalLoadBalancerConfig != nil {
		return ptr.Deref(lbSpec.ExternalLoadBalancerConfig.Name, infrav1.APIServerRoleTagValue)
	}
	return infrav1.APIServerRoleTagValue
}

// getLoadBalancingMode returns the appropriate balancing mode for the global
// backend service. When an internal passthrough LB is created alongside an
// external one (InternalExternal), they share instance groups: the internal LB
// only supports CONNECTION, and UTILIZATION cannot be mixed with another mode.
// See https://cloud.google.com/load-balancing/docs/backend-service#balancing-mode-lb
func getLoadBalancingMode(lbType infrav1.LoadBalancerType) loadBalancingMode {
	if lbType == infrav1.InternalExternal {
		return loadBalancingModeConnection
	}
	return loadBalancingModeUtilization
}

// createBackends builds Backend specs for the given instance groups.
// maxConnections is applied as-is to each Backend; pass 0 to leave the field
// unset (required for INTERNAL passthrough backend services — GCP rejects
// maxConnections on those).
func createBackends(instancegroups []*compute.InstanceGroup, mode loadBalancingMode, maxConnections int64) []*compute.Backend {
	backends := make([]*compute.Backend, 0, len(instancegroups))
	for _, group := range instancegroups {
		backends = append(backends, &compute.Backend{
			BalancingMode:  string(mode),
			Group:          group.SelfLink,
			MaxConnections: maxConnections,
		})
	}
	return backends
}
