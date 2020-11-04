/*
Copyright 2018 The Kubernetes Authors.

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

package compute

import (
	"fmt"
	"path"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/wait"
)

const (
	APIServerLoadBalancerProtocol            = "TCP"
	APIServerLoadBalancerHealthCheckProtocol = "SSL"
	APIServerLoadBalancerProxyHeader         = "NONE"
	APIServerLoadBalancerScheme              = "EXTERNAL"
	APIServerLoadBalancerIPVersion           = "IPV4"
	APIServerLoadBalancerBackendPortName     = "apiserver"
)

// ReconcileLoadbalancers reconciles the api server load balancer.
func (s *Service) ReconcileLoadbalancers() error {
	// Reconcile Health Check.
	healthCheckSpec := s.getAPIServerHealthCheckSpec()
	healthCheck, err := s.healthchecks.Get(s.scope.Project(), healthCheckSpec.Name).Do()
	if gcperrors.IsNotFound(err) {
		op, err := s.healthchecks.Insert(s.scope.Project(), healthCheckSpec).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create health check")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create health check")
		}
		healthCheck, err = s.healthchecks.Get(s.scope.Project(), healthCheckSpec.Name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to describe health check")
		}
	} else if err != nil {
		return errors.Wrapf(err, "failed to describe health check")
	}

	s.scope.Network().APIServerHealthCheck = pointer.StringPtr(healthCheck.SelfLink)

	// Reconcile Backend Service.
	backendServiceSpec := s.getAPIServerBackendServiceSpec()
	backendService, err := s.backendservices.Get(s.scope.Project(), backendServiceSpec.Name).Do()
	if gcperrors.IsNotFound(err) {
		op, err := s.backendservices.Insert(s.scope.Project(), backendServiceSpec).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create backend service")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create backend service")
		}
		backendService, err = s.backendservices.Get(s.scope.Project(), backendServiceSpec.Name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to describe backend service")
		}
	} else if err != nil {
		return errors.Wrapf(err, "failed to describe backend service")
	}

	s.scope.Network().APIServerBackendService = pointer.StringPtr(backendService.SelfLink)

	// Reconcile Target Proxy.
	targetProxySpec := s.getAPIServerTargetProxySpec()
	targetProxy, err := s.targetproxies.Get(s.scope.Project(), targetProxySpec.Name).Do()
	if gcperrors.IsNotFound(err) {
		op, err := s.targetproxies.Insert(s.scope.Project(), targetProxySpec).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create target proxy")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create target proxy")
		}
		targetProxy, err = s.targetproxies.Get(s.scope.Project(), targetProxySpec.Name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to describe target proxy")
		}
	} else if err != nil {
		return errors.Wrapf(err, "failed to describe target proxy")
	}

	s.scope.Network().APIServerTargetProxy = pointer.StringPtr(targetProxy.SelfLink)

	// Reconcile Global IP Address.
	addressSpec := s.getAPIServerIPAddressSpec()
	address, err := s.addresses.Get(s.scope.Project(), addressSpec.Name).Do()
	if gcperrors.IsNotFound(err) {
		op, err := s.addresses.Insert(s.scope.Project(), addressSpec).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create global addresses")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create global addresses")
		}
		address, err = s.addresses.Get(s.scope.Project(), addressSpec.Name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to describe global addresses")
		}
	} else if err != nil {
		return errors.Wrapf(err, "failed to describe addresses")
	}

	s.scope.Network().APIServerAddress = pointer.StringPtr(address.Address)

	// Reconcile Forwarding Rules.
	forwardingRuleSpec := s.getAPIServerForwardingRuleSpec()
	forwardingRule, err := s.forwardingrules.Get(s.scope.Project(), forwardingRuleSpec.Name).Do()
	if gcperrors.IsNotFound(err) {
		op, err := s.forwardingrules.Insert(s.scope.Project(), forwardingRuleSpec).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create forwarding rules")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create forwarding rules")
		}
		forwardingRule, err = s.forwardingrules.Get(s.scope.Project(), forwardingRuleSpec.Name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to describe forwarding rules")
		}
	} else if err != nil {
		return errors.Wrapf(err, "failed to describe forwarding rules")
	}

	s.scope.Network().APIServerForwardingRule = pointer.StringPtr(forwardingRule.SelfLink)

	return nil
}

func (s *Service) UpdateBackendServices() error {
	// Refresh the instance groups available.
	if err := s.ReconcileInstanceGroups(); err != nil {
		return err
	}

	// Retrieve the spec and the current backend service.
	backendServiceSpec := s.getAPIServerBackendServiceSpec()
	backendService, err := s.backendservices.Get(s.scope.Project(), backendServiceSpec.Name).Do()
	if err != nil {
		return err
	}

	// Update backend service if the list of backends has changed in the spec.
	// This might happen if new instance groups for the control plane api server
	// are created in additional zones.
	if len(backendService.Backends) != len(backendServiceSpec.Backends) {
		backendService.Backends = backendServiceSpec.Backends
		op, err := s.backendservices.Update(s.scope.Project(), backendService.Name, backendService).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to update backend service")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to update backend service")
		}
	}

	return nil
}

func (s *Service) DeleteLoadbalancers() error {
	// Delete Forwarding Rules.
	if s.scope.Network().APIServerForwardingRule != nil {
		name := path.Base(*s.scope.Network().APIServerForwardingRule)
		op, err := s.forwardingrules.Delete(s.scope.Project(), name).Do()
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete forwarding rules")
			}
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to wait for delete forwarding rules operation")
		}
		s.scope.Network().APIServerForwardingRule = nil
	}

	// Delete Global IP.
	if s.scope.Network().APIServerAddress != nil {
		name := s.getAPIServerIPAddressSpec().Name
		op, err := s.addresses.Delete(s.scope.Project(), name).Do()
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete globalAddress resource")
			}
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to wait for delete globalAddress resource operation")
		}
		s.scope.Network().APIServerAddress = nil
	}

	// Delete Target Proxy.
	if s.scope.Network().APIServerTargetProxy != nil {
		name := path.Base(*s.scope.Network().APIServerTargetProxy)
		op, err := s.targetproxies.Delete(s.scope.Project(), name).Do()
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete target proxy")
			}
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to wait for delete target proxy operation")
		}
		s.scope.Network().APIServerTargetProxy = nil
	}

	// Delete Backend Service.
	if s.scope.Network().APIServerBackendService != nil {
		name := path.Base(*s.scope.Network().APIServerBackendService)
		op, err := s.backendservices.Delete(s.scope.Project(), name).Do()
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete backend service")
			}
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to wait for delete backend service operation")
		}
		s.scope.Network().APIServerBackendService = nil
	}

	// Delete Health Check.
	if s.scope.Network().APIServerHealthCheck != nil {
		name := path.Base(*s.scope.Network().APIServerHealthCheck)
		op, err := s.healthchecks.Delete(s.scope.Project(), name).Do()
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete health check")
			}
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to wait for delete health check operation")
		}
		s.scope.Network().APIServerHealthCheck = nil
	}

	return nil
}

func (s *Service) getAPIServerHealthCheckSpec() *compute.HealthCheck {
	return &compute.HealthCheck{
		Name: fmt.Sprintf("%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue),
		Type: APIServerLoadBalancerHealthCheckProtocol,
		SslHealthCheck: &compute.SSLHealthCheck{
			Port:              s.scope.LoadBalancerBackendPort(),
			PortSpecification: "USE_FIXED_PORT",
		},
		CheckIntervalSec:   10,
		TimeoutSec:         5,
		HealthyThreshold:   5,
		UnhealthyThreshold: 3,
	}
}

func (s *Service) getAPIServerBackendServiceSpec() *compute.BackendService {
	res := &compute.BackendService{
		Name:                fmt.Sprintf("%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue),
		LoadBalancingScheme: APIServerLoadBalancerScheme,
		PortName:            APIServerLoadBalancerBackendPortName,
		Protocol:            APIServerLoadBalancerProtocol,
		TimeoutSec:          int64((10 * time.Minute).Seconds()),
		HealthChecks: []string{
			*s.scope.Network().APIServerHealthCheck,
		},
	}

	for _, groupSelfLink := range s.scope.Network().APIServerInstanceGroups {
		res.Backends = append(res.Backends, &compute.Backend{
			BalancingMode: "UTILIZATION",
			Group:         groupSelfLink,
		})
	}

	return res
}

func (s *Service) getAPIServerTargetProxySpec() *compute.TargetTcpProxy {
	return &compute.TargetTcpProxy{
		Name:        fmt.Sprintf("%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue),
		ProxyHeader: APIServerLoadBalancerProxyHeader,
		Service:     *s.scope.Network().APIServerBackendService,
	}
}

func (s *Service) getAPIServerIPAddressSpec() *compute.Address {
	return &compute.Address{
		Name:        fmt.Sprintf("%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue),
		AddressType: APIServerLoadBalancerScheme,
		IpVersion:   APIServerLoadBalancerIPVersion,
	}
}

func (s *Service) getAPIServerForwardingRuleSpec() *compute.ForwardingRule {
	frontendPortRange := fmt.Sprintf("%d-%d", s.scope.LoadBalancerFrontendPort(), s.scope.LoadBalancerFrontendPort())
	return &compute.ForwardingRule{
		Name:                fmt.Sprintf("%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue),
		IPAddress:           *s.scope.Network().APIServerAddress,
		IPProtocol:          APIServerLoadBalancerProtocol,
		LoadBalancingScheme: APIServerLoadBalancerScheme,
		PortRange:           frontendPortRange,
		Target:              *s.scope.Network().APIServerTargetProxy,
	}
}
