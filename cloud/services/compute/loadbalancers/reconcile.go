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

package loadbalancers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// loadBalancingMode describes the load balancing mode that the backend performs.
type loadBalancingMode string

const (
	// Utilization determines how the traffic load is spread based on the
	// utilization of instances.
	loadBalancingModeUtilization = loadBalancingMode("UTILIZATION")

	// Connection determines how the traffic load is spread based on the
	// total number of connections that a backend can handle. This is
	// only mode available for passthrough Load Balancers.
	loadBalancingModeConnection = loadBalancingMode("CONNECTION")

	loadBalanceTrafficInternal = "INTERNAL"
	loadBalanceTrafficExternal = "EXTERNAL"
)

// isRegionalExternalLoadBalancer returns true if the load balancer type
// requires regional external resources (for GCD environments).
func isRegionalExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
	return lbType == infrav1.RegionalExternal ||
		lbType == infrav1.RegionalInternalExternal
}

// shouldCreateExternalLoadBalancer returns true if an external load balancer
// should be created for the given load balancer type.
func shouldCreateExternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
	return lbType == infrav1.External ||
		lbType == infrav1.InternalExternal ||
		lbType == infrav1.RegionalExternal ||
		lbType == infrav1.RegionalInternalExternal
}

// shouldCreateInternalLoadBalancer returns true if an internal load balancer
// should be created for the given load balancer type.
func shouldCreateInternalLoadBalancer(lbType infrav1.LoadBalancerType) bool {
	return lbType == infrav1.Internal ||
		lbType == infrav1.InternalExternal ||
		lbType == infrav1.RegionalInternalExternal
}

// getExternalLoadBalancerName returns the name to use for external load balancer resources.
// Returns the custom name from configuration if set, otherwise returns the default API server name.
func getExternalLoadBalancerName(lbSpec infrav1.LoadBalancerSpec) string {
	if lbSpec.ExternalLoadBalancer != nil && lbSpec.ExternalLoadBalancer.Name != nil {
		return *lbSpec.ExternalLoadBalancer.Name
	}
	return infrav1.APIServerRoleTagValue
}

// getInternalLoadBalancerName returns the name to use for internal load balancer resources.
// Returns the custom name from configuration if set, otherwise returns the default internal name.
func getInternalLoadBalancerName(lbSpec infrav1.LoadBalancerSpec) string {
	if lbSpec.InternalLoadBalancer != nil && lbSpec.InternalLoadBalancer.Name != nil {
		return *lbSpec.InternalLoadBalancer.Name
	}
	return infrav1.InternalRoleTagValue
}

// getLoadBalancingMode returns the appropriate balancing mode based on the load balancer type.
// RegionalInternalExternal uses CONNECTION mode to match internal LB requirements.
// All other external LBs use UTILIZATION mode.
func getLoadBalancingMode(lbType infrav1.LoadBalancerType) loadBalancingMode {
	if lbType == infrav1.RegionalInternalExternal || lbType == infrav1.InternalExternal {
		return loadBalancingModeConnection
	}
	return loadBalancingModeUtilization
}

// createBackends creates backend instances for the given instance groups with the specified balancing mode.
func createBackends(instancegroups []*compute.InstanceGroup, mode loadBalancingMode) []*compute.Backend {
	backends := make([]*compute.Backend, 0, len(instancegroups))
	for _, group := range instancegroups {
		be := &compute.Backend{
			BalancingMode: string(mode),
			Group:         group.SelfLink,
		}
		if mode == loadBalancingModeConnection {
			// Set max connections to a reasonable limit based
			// on database max connections https://cloud.google.com/sql/docs/postgres/flags#postgres-m
			be.MaxConnections = 1000
		}
		backends = append(backends, be)
	}
	return backends
}

// Reconcile reconcile cluster control-plane loadbalancer components.
func (s *Service) Reconcile(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling loadbalancer resources")

	// Creates instance groups used by load balancer(s)
	instancegroups, err := s.createOrGetInstanceGroups(ctx)
	if err != nil {
		return err
	}

	lbSpec := s.scope.LoadBalancer()
	lbType := ptr.Deref(lbSpec.LoadBalancerType, infrav1.External)

	// Create External Load Balancer (Global or Regional based on type)
	if shouldCreateExternalLoadBalancer(lbType) {
		if isRegionalExternalLoadBalancer(lbType) {
			// Create Regional External Proxy Load Balancer for GCD
			if err = s.createRegionalExternalLoadBalancer(ctx, lbType, instancegroups); err != nil {
				return err
			}
		} else {
			// Create Global External Proxy Load Balancer
			if err = s.createExternalLoadBalancer(ctx, lbType, instancegroups); err != nil {
				return err
			}
		}
	}

	// Create a Regional Internal Passthrough Load Balancer if configured
	if shouldCreateInternalLoadBalancer(lbType) {
		name := getInternalLoadBalancerName(lbSpec)
		if err = s.createInternalLoadBalancer(ctx, name, lbType, instancegroups); err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes cluster control-plane loadbalancer components.
func (s *Service) Delete(ctx context.Context) error {
	log := log.FromContext(ctx)
	var allErrs []error
	lbSpec := s.scope.LoadBalancer()
	lbType := ptr.Deref(lbSpec.LoadBalancerType, infrav1.External)

	// Delete External Load Balancer (Global or Regional based on type)
	if shouldCreateExternalLoadBalancer(lbType) {
		if isRegionalExternalLoadBalancer(lbType) {
			if err := s.deleteRegionalExternalLoadBalancer(ctx); err != nil {
				allErrs = append(allErrs, err)
			}
		} else {
			if err := s.deleteExternalLoadBalancer(ctx); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	if shouldCreateInternalLoadBalancer(lbType) {
		name := getInternalLoadBalancerName(lbSpec)
		if err := s.deleteInternalLoadBalancer(ctx, name); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	if err := s.deleteInstanceGroups(ctx); err != nil {
		log.Error(err, "Error deleting instancegroup")
		allErrs = append(allErrs, err)
	}

	return errors.Join(allErrs...)
}

func (s *Service) deleteExternalLoadBalancer(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Deleting external loadbalancer resources")
	name := infrav1.APIServerRoleTagValue
	if err := s.deleteForwardingRule(ctx, name); err != nil {
		return fmt.Errorf("deleting forwarding rule: %w", err)
	}
	s.scope.Network().APIServerForwardingRule = nil

	if err := s.deleteAddress(ctx, name); err != nil {
		return fmt.Errorf("deleting address: %w", err)
	}
	s.scope.Network().APIServerAddress = nil

	if err := s.deleteTargetTCPProxy(ctx); err != nil {
		return fmt.Errorf("deleting target TCP proxy: %w", err)
	}
	s.scope.Network().APIServerTargetProxy = nil

	if err := s.deleteBackendService(ctx, name); err != nil {
		return fmt.Errorf("deleting backend service: %w", err)
	}
	s.scope.Network().APIServerBackendService = nil

	if err := s.deleteHealthCheck(ctx, name); err != nil {
		return fmt.Errorf("deleting health check: %w", err)
	}
	s.scope.Network().APIServerHealthCheck = nil

	return nil
}

func (s *Service) deleteInternalLoadBalancer(ctx context.Context, name string) error {
	log := log.FromContext(ctx)
	log.Info("Deleting internal loadbalancer resources")
	if err := s.deleteRegionalForwardingRule(ctx, name); err != nil {
		return fmt.Errorf("deleting forwarding rule: %w", err)
	}
	s.scope.Network().APIInternalForwardingRule = nil

	if err := s.deleteInternalAddress(ctx, name); err != nil {
		return fmt.Errorf("deleting internal address: %w", err)
	}
	s.scope.Network().APIInternalAddress = nil

	if err := s.deleteRegionalBackendService(ctx, name); err != nil {
		return fmt.Errorf("deleting regional backend service: %w", err)
	}
	s.scope.Network().APIInternalBackendService = nil

	if err := s.deleteRegionalHealthCheck(ctx, name); err != nil {
		return fmt.Errorf("deleting regional health check: %w", err)
	}
	s.scope.Network().APIInternalHealthCheck = nil

	return nil
}

func (s *Service) deleteRegionalExternalLoadBalancer(ctx context.Context) error {
	log := log.FromContext(ctx)
	log.Info("Deleting regional external loadbalancer resources")
	lbSpec := s.scope.LoadBalancer()
	name := getExternalLoadBalancerName(lbSpec)

	// Delete in reverse order of creation
	if err := s.deleteRegionalForwardingRule(ctx, name); err != nil {
		return fmt.Errorf("deleting regional forwarding rule: %w", err)
	}
	s.scope.Network().APIServerForwardingRule = nil

	if err := s.deleteRegionalAddress(ctx, name); err != nil {
		return fmt.Errorf("deleting regional address: %w", err)
	}
	s.scope.Network().APIServerAddress = nil

	if err := s.deleteRegionalTargetTCPProxy(ctx); err != nil {
		return fmt.Errorf("deleting regional target TCP proxy: %w", err)
	}
	s.scope.Network().APIServerTargetProxy = nil

	if err := s.deleteRegionalBackendService(ctx, name); err != nil {
		return fmt.Errorf("deleting regional backend service: %w", err)
	}
	s.scope.Network().APIServerBackendService = nil

	if err := s.deleteRegionalHealthCheck(ctx, name); err != nil {
		return fmt.Errorf("deleting regional health check: %w", err)
	}
	s.scope.Network().APIServerHealthCheck = nil

	return nil
}

// createExternalLoadBalancer creates the components for a Global External Proxy LoadBalancer.
func (s *Service) createExternalLoadBalancer(ctx context.Context, lbType infrav1.LoadBalancerType, instancegroups []*compute.InstanceGroup) error {
	name := infrav1.APIServerRoleTagValue
	healthcheck, err := s.createOrGetHealthCheck(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerHealthCheck = ptr.To[string](healthcheck.SelfLink)

	// Determine balancing mode based on load balancer type
	mode := getLoadBalancingMode(lbType)
	backendsvc, err := s.createOrGetBackendService(ctx, name, mode, instancegroups, healthcheck)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerBackendService = ptr.To[string](backendsvc.SelfLink)

	// Create TargetTCPProxy for Proxy Load Balancer
	target, err := s.createOrGetTargetTCPProxy(ctx, backendsvc)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerTargetProxy = ptr.To[string](target.SelfLink)

	addr, err := s.createOrGetAddress(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerAddress = ptr.To[string](addr.SelfLink)
	endpoint := s.scope.ControlPlaneEndpoint()
	endpoint.Host = addr.Address
	s.scope.SetControlPlaneEndpoint(endpoint)

	forwarding, err := s.createOrGetForwardingRule(ctx, name, target, addr)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerForwardingRule = ptr.To[string](forwarding.SelfLink)

	return nil
}

// createInternalLoadBalancer creates the components for a Regional Internal Passthrough LoadBalancer.
// Since this is a passthrough LoadBalancer the TargetTCPProxy resource is not created.
func (s *Service) createInternalLoadBalancer(ctx context.Context, name string, lbType infrav1.LoadBalancerType, instancegroups []*compute.InstanceGroup) error {
	healthcheck, err := s.createOrGetRegionalHealthCheck(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIInternalHealthCheck = ptr.To[string](healthcheck.SelfLink)

	backendsvc, err := s.createOrGetRegionalBackendService(ctx, name, instancegroups, healthcheck)
	if err != nil {
		return err
	}
	s.scope.Network().APIInternalBackendService = ptr.To[string](backendsvc.SelfLink)

	// Create an address on internal subnet.
	addr, err := s.createOrGetInternalAddress(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIInternalAddress = ptr.To[string](addr.SelfLink)
	if lbType == infrav1.Internal {
		// If only creating an internal Load Balancer, set the control plane endpoint
		endpoint := s.scope.ControlPlaneEndpoint()
		endpoint.Host = addr.Address
		s.scope.SetControlPlaneEndpoint(endpoint)
	}

	// Create a regional forwarding rule to the backend service
	forwarding, err := s.createOrGetRegionalForwardingRule(ctx, name, backendsvc, addr)
	if err != nil {
		return err
	}
	s.scope.Network().APIInternalForwardingRule = ptr.To[string](forwarding.SelfLink)

	return nil
}

// createRegionalExternalLoadBalancer creates the components for a Regional External Proxy LoadBalancer.
// This is required for GCD (Google Cloud Distributed/Sovereign Cloud) environments which do not support
// global load balancer resources.
func (s *Service) createRegionalExternalLoadBalancer(ctx context.Context, lbType infrav1.LoadBalancerType, instancegroups []*compute.InstanceGroup) error {
	lbSpec := s.scope.LoadBalancer()
	name := getExternalLoadBalancerName(lbSpec)

	// Step 1: Create Regional Health Check
	healthcheck, err := s.createOrGetRegionalHealthCheck(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerHealthCheck = ptr.To[string](healthcheck.SelfLink)

	// Determine balancing mode based on load balancer type
	mode := getLoadBalancingMode(lbType)

	// Step 2: Create Regional Backend Service (with EXTERNAL load balancing scheme)
	backendsvc, err := s.createOrGetRegionalBackendServiceExternal(ctx, name, mode, instancegroups, healthcheck)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerBackendService = ptr.To[string](backendsvc.SelfLink)

	// Step 3: Create Regional TargetTCPProxy (CRITICAL: This is the key component for GCD)
	target, err := s.createOrGetRegionalTargetTCPProxy(ctx, backendsvc)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerTargetProxy = ptr.To[string](target.SelfLink)

	// Step 4: Create Regional Address (with EXTERNAL address type)
	addr, err := s.createOrGetRegionalAddress(ctx, name)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerAddress = ptr.To[string](addr.SelfLink)

	// Set control plane endpoint to the external address
	endpoint := s.scope.ControlPlaneEndpoint()
	endpoint.Host = addr.Address
	s.scope.SetControlPlaneEndpoint(endpoint)

	// Step 5: Create Regional Forwarding Rule (points to Target TCP Proxy, not Backend Service)
	forwarding, err := s.createOrGetRegionalForwardingRuleWithProxy(ctx, name, target, addr)
	if err != nil {
		return err
	}
	s.scope.Network().APIServerForwardingRule = ptr.To[string](forwarding.SelfLink)

	return nil
}

func (s *Service) createOrGetInstanceGroups(ctx context.Context) ([]*compute.InstanceGroup, error) {
	log := log.FromContext(ctx)
	zones := s.scope.FailureDomains()

	groups := make([]*compute.InstanceGroup, 0, len(zones))
	groupsMap := s.scope.Network().APIServerInstanceGroups
	if groupsMap == nil {
		groupsMap = make(map[string]string)
	}

	for _, zone := range zones {
		instancegroupSpec := s.scope.InstanceGroupSpec(zone)
		log.V(2).Info("Looking for instancegroup in zone", "zone", zone, "name", instancegroupSpec.Name)
		instancegroup, err := s.instancegroups.Get(ctx, meta.ZonalKey(instancegroupSpec.Name, zone))
		if err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error looking for instancegroup in zone", "zone", zone)
				return groups, err
			}

			log.V(2).Info("Creating instancegroup in zone", "zone", zone, "name", instancegroupSpec.Name)
			if err := s.instancegroups.Insert(ctx, meta.ZonalKey(instancegroupSpec.Name, zone), instancegroupSpec); err != nil {
				log.Error(err, "Error creating instancegroup", "name", instancegroupSpec.Name)
				return groups, err
			}

			instancegroup, err = s.instancegroups.Get(ctx, meta.ZonalKey(instancegroupSpec.Name, zone))
			if err != nil {
				return groups, err
			}
		}

		groups = append(groups, instancegroup)
		groupsMap[zone] = instancegroup.SelfLink
	}

	s.scope.Network().APIServerInstanceGroups = groupsMap
	return groups, nil
}

func (s *Service) createOrGetHealthCheck(ctx context.Context, lbname string) (*compute.HealthCheck, error) {
	log := log.FromContext(ctx)
	healthcheckSpec := s.scope.HealthCheckSpec(lbname)
	log.V(2).Info("Looking for healthcheck", "name", healthcheckSpec.Name)
	key := meta.GlobalKey(healthcheckSpec.Name)
	healthcheck, err := s.healthchecks.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for healthcheck", "name", healthcheckSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a healthcheck", "name", healthcheckSpec.Name)
		if err := s.healthchecks.Insert(ctx, key, healthcheckSpec); err != nil {
			log.Error(err, "Error creating a healthcheck", "name", healthcheckSpec.Name)
			return nil, err
		}

		healthcheck, err = s.healthchecks.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return healthcheck, nil
}

func (s *Service) createOrGetRegionalHealthCheck(ctx context.Context, lbname string) (*compute.HealthCheck, error) {
	log := log.FromContext(ctx)
	healthcheckSpec := s.scope.HealthCheckSpec(lbname)
	healthcheckSpec.Region = s.scope.Region()
	log.V(2).Info("Looking for regional healthcheck", "name", healthcheckSpec.Name)
	key := meta.RegionalKey(healthcheckSpec.Name, s.scope.Region())
	healthcheck, err := s.regionalhealthchecks.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional healthcheck", "name", healthcheckSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional healthcheck", "name", healthcheckSpec.Name)
		if err := s.regionalhealthchecks.Insert(ctx, key, healthcheckSpec); err != nil {
			log.Error(err, "Error creating a regional healthcheck", "name", healthcheckSpec.Name)
			return nil, err
		}

		healthcheck, err = s.regionalhealthchecks.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return healthcheck, nil
}

func (s *Service) createOrGetBackendService(ctx context.Context, lbname string, mode loadBalancingMode, instancegroups []*compute.InstanceGroup, healthcheck *compute.HealthCheck) (*compute.BackendService, error) {
	log := log.FromContext(ctx)
	backends := createBackends(instancegroups, mode)

	backendsvcSpec := s.scope.BackendServiceSpec(lbname)
	backendsvcSpec.Backends = backends
	backendsvcSpec.HealthChecks = []string{healthcheck.SelfLink}

	key := meta.GlobalKey(backendsvcSpec.Name)
	backendsvc, err := s.backendservices.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a backendservice", "name", backendsvcSpec.Name)
		if err := s.backendservices.Insert(ctx, key, backendsvcSpec); err != nil {
			log.Error(err, "Error creating a backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}

		backendsvc, err = s.backendservices.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	if len(backendsvc.Backends) != len(backendsvcSpec.Backends) {
		log.V(2).Info("Updating a backendservice", "name", backendsvcSpec.Name)
		backendsvc.Backends = backendsvcSpec.Backends
		if err := s.backendservices.Update(ctx, key, backendsvc); err != nil {
			log.Error(err, "Error updating a backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}
	}

	return backendsvc, nil
}

// createOrGetRegionalBackendService is used for internal passthrough load balancers.
func (s *Service) createOrGetRegionalBackendService(ctx context.Context, lbname string, instancegroups []*compute.InstanceGroup, healthcheck *compute.HealthCheck) (*compute.BackendService, error) {
	log := log.FromContext(ctx)
	// Always use connection mode for passthrough load balancer
	backends := createBackends(instancegroups, loadBalancingModeConnection)

	backendsvcSpec := s.scope.BackendServiceSpec(lbname)
	backendsvcSpec.Backends = backends
	backendsvcSpec.HealthChecks = []string{healthcheck.SelfLink}
	backendsvcSpec.Region = s.scope.Region()
	backendsvcSpec.LoadBalancingScheme = string(loadBalanceTrafficInternal)
	backendsvcSpec.PortName = ""
	network := s.scope.Network()
	if network.SelfLink != nil {
		backendsvcSpec.Network = *network.SelfLink
	}

	key := meta.RegionalKey(backendsvcSpec.Name, s.scope.Region())
	backendsvc, err := s.regionalbackendservices.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional backendservice", "name", backendsvcSpec.Name)
		if err := s.regionalbackendservices.Insert(ctx, key, backendsvcSpec); err != nil {
			log.Error(err, "Error creating a regional backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}

		backendsvc, err = s.regionalbackendservices.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	if len(backendsvc.Backends) != len(backendsvcSpec.Backends) {
		log.V(2).Info("Updating a regional backendservice", "name", backendsvcSpec.Name)
		backendsvc.Backends = backendsvcSpec.Backends
		if err := s.regionalbackendservices.Update(ctx, key, backendsvc); err != nil {
			log.Error(err, "Error updating a regional backendservice", "name", backendsvcSpec.Name)
			return nil, err
		}
	}

	return backendsvc, nil
}

// createOrGetRegionalBackendServiceExternal creates a regional backend service for external load balancers.
// This is different from createOrGetRegionalBackendService which is for internal passthrough load balancers
// and uses INTERNAL load balancing scheme with a network field.
func (s *Service) createOrGetRegionalBackendServiceExternal(ctx context.Context, lbname string, mode loadBalancingMode, instancegroups []*compute.InstanceGroup, healthcheck *compute.HealthCheck) (*compute.BackendService, error) {
	log := log.FromContext(ctx)
	backends := createBackends(instancegroups, mode)

	backendsvcSpec := s.scope.BackendServiceSpec(lbname)
	backendsvcSpec.Backends = backends
	backendsvcSpec.HealthChecks = []string{healthcheck.SelfLink}
	backendsvcSpec.Region = s.scope.Region()
	// External regional load balancer uses EXTERNAL scheme (not INTERNAL)
	backendsvcSpec.LoadBalancingScheme = loadBalanceTrafficExternal
	// Do NOT set Network field for external load balancers (only for internal)
	backendsvcSpec.PortName = ""

	key := meta.RegionalKey(backendsvcSpec.Name, s.scope.Region())
	backendsvc, err := s.regionalbackendservices.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional backend service for external LB", "name", backendsvcSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional backend service for external LB", "name", backendsvcSpec.Name)
		if err := s.regionalbackendservices.Insert(ctx, key, backendsvcSpec); err != nil {
			log.Error(err, "Error creating a regional backend service for external LB", "name", backendsvcSpec.Name)
			return nil, err
		}

		backendsvc, err = s.regionalbackendservices.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	if len(backendsvc.Backends) != len(backendsvcSpec.Backends) {
		log.V(2).Info("Updating a regional backend service for external LB", "name", backendsvcSpec.Name)
		backendsvc.Backends = backendsvcSpec.Backends
		if err := s.regionalbackendservices.Update(ctx, key, backendsvc); err != nil {
			log.Error(err, "Error updating a regional backend service for external LB", "name", backendsvcSpec.Name)
			return nil, err
		}
	}

	return backendsvc, nil
}

func (s *Service) createOrGetTargetTCPProxy(ctx context.Context, service *compute.BackendService) (*compute.TargetTcpProxy, error) {
	log := log.FromContext(ctx)
	targetSpec := s.scope.TargetTCPProxySpec()
	targetSpec.Service = service.SelfLink
	key := meta.GlobalKey(targetSpec.Name)
	target, err := s.targettcpproxies.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for targettcpproxy", "name", targetSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a targettcpproxy", "name", targetSpec.Name)
		if err := s.targettcpproxies.Insert(ctx, key, targetSpec); err != nil {
			log.Error(err, "Error creating a targettcpproxy", "name", targetSpec.Name)
			return nil, err
		}

		target, err = s.targettcpproxies.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return target, nil
}

// createOrGetRegionalTargetTCPProxy creates or gets a regional TargetTCPProxy.
// This is the CRITICAL missing piece for regional external load balancers in GCD.
// Unlike the global version, this uses meta.RegionalKey and sets the Region field.
func (s *Service) createOrGetRegionalTargetTCPProxy(ctx context.Context, service *compute.BackendService) (*compute.TargetTcpProxy, error) {
	log := log.FromContext(ctx)
	targetSpec := s.scope.TargetTCPProxySpec()
	targetSpec.Service = service.SelfLink
	targetSpec.Region = s.scope.Region()

	key := meta.RegionalKey(targetSpec.Name, s.scope.Region())
	target, err := s.regionaltargettcpproxies.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional targettcpproxy", "name", targetSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional targettcpproxy", "name", targetSpec.Name)
		if err := s.regionaltargettcpproxies.Insert(ctx, key, targetSpec); err != nil {
			log.Error(err, "Error creating a regional targettcpproxy", "name", targetSpec.Name)
			return nil, err
		}

		target, err = s.regionaltargettcpproxies.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return target, nil
}

// createOrGetAddress is used to obtain a Global address.
func (s *Service) createOrGetAddress(ctx context.Context, lbname string) (*compute.Address, error) {
	log := log.FromContext(ctx)
	addrSpec := s.scope.AddressSpec(lbname)
	log.V(2).Info("Looking for address", "name", addrSpec.Name)
	key := meta.GlobalKey(addrSpec.Name)
	addr, err := s.addresses.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for address", "name", addrSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating an address", "name", addrSpec.Name)
		if err := s.addresses.Insert(ctx, key, addrSpec); err != nil {
			log.Error(err, "Error creating an address", "name", addrSpec.Name)
			return nil, err
		}

		addr, err = s.addresses.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return addr, nil
}

// createOrGetInternalAddress is used to obtain an internal address.
func (s *Service) createOrGetInternalAddress(ctx context.Context, lbname string) (*compute.Address, error) {
	log := log.FromContext(ctx)
	addrSpec := s.scope.AddressSpec(lbname)
	addrSpec.AddressType = string(loadBalanceTrafficInternal)
	addrSpec.Region = s.scope.Region()
	subnet, err := s.getSubnet(ctx)
	if err != nil {
		log.Error(err, "Error getting subnet for Internal Load Balancer")
		return nil, err
	}
	lbSpec := s.scope.LoadBalancer()
	if lbSpec.InternalLoadBalancer != nil && lbSpec.InternalLoadBalancer.IPAddress != nil {
		// If an IP address is configured, use it instead of creating a new one.
		addrSpec.Address = *lbSpec.InternalLoadBalancer.IPAddress
	}
	addrSpec.Subnetwork = subnet.SelfLink
	addrSpec.Purpose = "GCE_ENDPOINT"
	log.V(2).Info("Looking for internal address", "name", addrSpec.Name)
	key := meta.RegionalKey(addrSpec.Name, s.scope.Region())
	addr, err := s.internaladdresses.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for internal address", "name", addrSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating an internal address", "name", addrSpec.Name)
		if err := s.internaladdresses.Insert(ctx, key, addrSpec); err != nil {
			log.Error(err, "Error creating an internal address", "name", addrSpec.Name)
			return nil, err
		}

		addr, err = s.internaladdresses.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return addr, nil
}

// createOrGetRegionalAddress creates or gets a regional address for external load balancers.
// This is different from createOrGetInternalAddress which sets AddressType to INTERNAL,
// requires a subnet, and sets a Purpose field. External addresses use EXTERNAL type,
// do not require a subnet, and do not set a Purpose.
func (s *Service) createOrGetRegionalAddress(ctx context.Context, lbname string) (*compute.Address, error) {
	log := log.FromContext(ctx)
	addrSpec := s.scope.AddressSpec(lbname)
	addrSpec.Region = s.scope.Region()
	addrSpec.AddressType = loadBalanceTrafficExternal

	lbSpec := s.scope.LoadBalancer()
	if lbSpec.ExternalLoadBalancer != nil && lbSpec.ExternalLoadBalancer.IPAddress != nil {
		// If an IP address is configured, use it instead of creating a new one
		addrSpec.Address = *lbSpec.ExternalLoadBalancer.IPAddress
	}

	log.V(2).Info("Looking for regional external address", "name", addrSpec.Name)
	key := meta.RegionalKey(addrSpec.Name, s.scope.Region())
	addr, err := s.addresses.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional external address", "name", addrSpec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional external address", "name", addrSpec.Name)
		if err := s.addresses.Insert(ctx, key, addrSpec); err != nil {
			log.Error(err, "Error creating a regional external address", "name", addrSpec.Name)
			return nil, err
		}

		addr, err = s.addresses.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	return addr, nil
}

// createOrGetForwardingRule is used obtain a Global ForwardingRule.
func (s *Service) createOrGetForwardingRule(ctx context.Context, lbname string, target *compute.TargetTcpProxy, addr *compute.Address) (*compute.ForwardingRule, error) {
	log := log.FromContext(ctx)
	spec := s.scope.ForwardingRuleSpec(lbname)
	spec.Target = target.SelfLink
	spec.IPAddress = addr.SelfLink

	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Looking for forwardingrule", "name", spec.Name)
	forwarding, err := s.forwardingrules.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for forwardingrule", "name", spec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a forwardingrule", "name", spec.Name)
		if err := s.forwardingrules.Insert(ctx, key, spec); err != nil {
			log.Error(err, "Error creating a forwardingrule", "name", spec.Name)
			return nil, err
		}

		forwarding, err = s.forwardingrules.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	// Labels on ForwardingRules must be added after resource is created
	labels := s.scope.AdditionalLabels()
	if !labels.Equals(forwarding.Labels) {
		setLabelsRequest := &compute.GlobalSetLabelsRequest{
			LabelFingerprint: forwarding.LabelFingerprint,
			Labels:           labels,
		}
		if err = s.forwardingrules.SetLabels(ctx, key, setLabelsRequest); err != nil {
			return nil, err
		}
	}

	return forwarding, nil
}

// createOrGetRegionalForwardingRule is used to obtain a Regional ForwardingRule.
func (s *Service) createOrGetRegionalForwardingRule(ctx context.Context, lbname string, backendSvc *compute.BackendService, addr *compute.Address) (*compute.ForwardingRule, error) {
	log := log.FromContext(ctx)
	spec := s.scope.ForwardingRuleSpec(lbname)
	spec.LoadBalancingScheme = string(loadBalanceTrafficInternal)
	spec.Region = s.scope.Region()
	spec.BackendService = backendSvc.SelfLink
	lbSpec := s.scope.LoadBalancer()
	if lbSpec.InternalLoadBalancer != nil && lbSpec.InternalLoadBalancer.InternalAccess == infrav1.InternalAccessGlobal {
		spec.AllowGlobalAccess = true
	}
	// Ports is used instead or PortRange for passthrough Load Balancer
	// Configure ports for k8s API to match the external API which is the first port of range
	var ports []string
	portList := strings.Split(spec.PortRange, "-")
	ports = append(ports, portList[0])
	// Also configure ignition port
	ports = append(ports, "22623")
	spec.Ports = ports
	spec.PortRange = ""
	subnet, err := s.getSubnet(ctx)
	if err != nil {
		log.Error(err, "Error getting subnet for regional forwardingrule")
		return nil, err
	}
	spec.Subnetwork = subnet.SelfLink
	spec.IPAddress = addr.SelfLink

	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Looking for regional forwardingrule", "name", spec.Name)
	forwarding, err := s.regionalforwardingrules.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional forwardingrule", "name", spec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional forwardingrule", "name", spec.Name)
		if err := s.regionalforwardingrules.Insert(ctx, key, spec); err != nil {
			log.Error(err, "Error creating a regional forwardingrule", "name", spec.Name)
			return nil, err
		}

		forwarding, err = s.regionalforwardingrules.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	// Labels on ForwardingRules must be added after resource is created
	labels := s.scope.AdditionalLabels()
	if !labels.Equals(forwarding.Labels) {
		setLabelsRequest := &compute.RegionSetLabelsRequest{
			LabelFingerprint: forwarding.LabelFingerprint,
			Labels:           labels,
		}
		if err = s.regionalforwardingrules.SetLabels(ctx, key, setLabelsRequest); err != nil {
			return nil, err
		}
	}

	return forwarding, nil
}

// createOrGetRegionalForwardingRuleWithProxy creates a regional forwarding rule that points to a target proxy.
// This is for regional external load balancers, different from createOrGetRegionalForwardingRule which
// is for internal passthrough load balancers that point directly to a backend service.
// Key differences: EXTERNAL scheme, points to Target (not BackendService), uses PortRange (not Ports).
func (s *Service) createOrGetRegionalForwardingRuleWithProxy(ctx context.Context, lbname string, target *compute.TargetTcpProxy, addr *compute.Address) (*compute.ForwardingRule, error) {
	log := log.FromContext(ctx)
	spec := s.scope.ForwardingRuleSpec(lbname)
	spec.Region = s.scope.Region()
	spec.Target = target.SelfLink
	spec.IPAddress = addr.SelfLink
	spec.LoadBalancingScheme = loadBalanceTrafficExternal
	// Use PortRange for proxy load balancer (not Ports array which is for passthrough)
	// spec.PortRange is already set by ForwardingRuleSpec

	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Looking for regional forwarding rule with proxy", "name", spec.Name)
	forwarding, err := s.regionalforwardingrules.Get(ctx, key)
	if err != nil {
		if !gcperrors.IsNotFound(err) {
			log.Error(err, "Error looking for regional forwarding rule with proxy", "name", spec.Name)
			return nil, err
		}

		log.V(2).Info("Creating a regional forwarding rule with proxy", "name", spec.Name)
		if err := s.regionalforwardingrules.Insert(ctx, key, spec); err != nil {
			log.Error(err, "Error creating a regional forwarding rule with proxy", "name", spec.Name)
			return nil, err
		}

		forwarding, err = s.regionalforwardingrules.Get(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	// Labels on ForwardingRules must be added after resource is created
	labels := s.scope.AdditionalLabels()
	if !labels.Equals(forwarding.Labels) {
		setLabelsRequest := &compute.RegionSetLabelsRequest{
			LabelFingerprint: forwarding.LabelFingerprint,
			Labels:           labels,
		}
		if err = s.regionalforwardingrules.SetLabels(ctx, key, setLabelsRequest); err != nil {
			return nil, err
		}
	}

	return forwarding, nil
}

func (s *Service) deleteForwardingRule(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.ForwardingRuleSpec(lbname)
	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Deleting a forwardingrule", "name", spec.Name)
	if err := s.forwardingrules.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error updating a forwardingrule", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteRegionalForwardingRule(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.ForwardingRuleSpec(lbname)
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting a regional forwardingrule", "name", spec.Name)
	if err := s.regionalforwardingrules.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error updating a regional forwardingrule", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteAddress(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.AddressSpec(lbname)
	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Deleting a address", "name", spec.Name)
	if err := s.addresses.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Service) deleteInternalAddress(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.AddressSpec(lbname)
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting an internal address", "name", spec.Name)
	if err := s.internaladdresses.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Service) deleteRegionalAddress(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.AddressSpec(lbname)
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting a regional address", "name", spec.Name)
	if err := s.addresses.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Service) deleteTargetTCPProxy(ctx context.Context) error {
	log := log.FromContext(ctx)
	spec := s.scope.TargetTCPProxySpec()
	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Deleting a targettcpproxy", "name", spec.Name)
	if err := s.targettcpproxies.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a targettcpproxy", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteRegionalTargetTCPProxy(ctx context.Context) error {
	log := log.FromContext(ctx)
	spec := s.scope.TargetTCPProxySpec()
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting a regional targettcpproxy", "name", spec.Name)
	if err := s.regionaltargettcpproxies.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a regional targettcpproxy", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteBackendService(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.BackendServiceSpec(lbname)
	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Deleting a backendservice", "name", spec.Name)
	if err := s.backendservices.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a backendservice", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteRegionalBackendService(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.BackendServiceSpec(lbname)
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting a regional backendservice", "name", spec.Name)
	if err := s.regionalbackendservices.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a regional backendservice", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteHealthCheck(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.HealthCheckSpec(lbname)
	key := meta.GlobalKey(spec.Name)
	log.V(2).Info("Deleting a healthcheck", "name", spec.Name)
	if err := s.healthchecks.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a healthcheck", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteRegionalHealthCheck(ctx context.Context, lbname string) error {
	log := log.FromContext(ctx)
	spec := s.scope.HealthCheckSpec(lbname)
	key := meta.RegionalKey(spec.Name, s.scope.Region())
	log.V(2).Info("Deleting a regional healthcheck", "name", spec.Name)
	if err := s.regionalhealthchecks.Delete(ctx, key); err != nil && !gcperrors.IsNotFound(err) {
		log.Error(err, "Error deleting a regional healthcheck", "name", spec.Name)
		return err
	}

	return nil
}

func (s *Service) deleteInstanceGroups(ctx context.Context) error {
	log := log.FromContext(ctx)
	for zone := range s.scope.Network().APIServerInstanceGroups {
		spec := s.scope.InstanceGroupSpec(zone)
		key := meta.ZonalKey(spec.Name, zone)
		log.V(2).Info("Deleting a instancegroup", "name", spec.Name)
		if err := s.instancegroups.Delete(ctx, key); err != nil {
			if !gcperrors.IsNotFound(err) {
				log.Error(err, "Error deleting a instancegroup", "name", spec.Name)
				return err
			}

			delete(s.scope.Network().APIServerInstanceGroups, zone)
		}
	}

	return nil
}

// getSubnet gets the subnet to use for an internal Load Balancer.
func (s *Service) getSubnet(ctx context.Context) (*compute.Subnetwork, error) {
	log := log.FromContext(ctx)
	cfgSubnet := ""
	lbSpec := s.scope.LoadBalancer()
	if lbSpec.InternalLoadBalancer != nil {
		cfgSubnet = ptr.Deref(lbSpec.InternalLoadBalancer.Subnet, "")
	}
	for _, subnetSpec := range s.scope.SubnetSpecs() {
		log.V(2).Info("Looking for subnet for load balancer", "name", subnetSpec.Name)
		region := subnetSpec.Region
		if region == "" {
			region = s.scope.Region()
		}

		subnetKey := meta.RegionalKey(subnetSpec.Name, region)
		subnet, err := s.subnets.Get(ctx, subnetKey)
		if err != nil {
			return nil, err
		}
		// Return subnet that matches configuration, or first one if not configured
		if cfgSubnet == "" || strings.HasSuffix(subnet.Name, cfgSubnet) {
			return subnet, nil
		}
	}

	return nil, errors.New("could not find subnet")
}
