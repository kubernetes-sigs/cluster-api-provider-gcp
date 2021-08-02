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
	"google.golang.org/api/container/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
)

// Service holds a collection of interfaces.
// The interfaces are broken down like this to group functions together.
// One alternative is to have a large list of functions from the gcp client.
type Service struct {
	scope *scope.GKEClusterScope
	project *container.ProjectsService

	// Helper clients for GCP.
	//instances       *compute.InstancesService
	//instancegroups  *compute.InstanceGroupsService
	//networks        *compute.NetworksService
	//subnetworks     *compute.SubnetworksService
	//healthchecks    *compute.HealthChecksService
	//backendservices *compute.BackendServicesService
	//targetproxies   *compute.TargetTcpProxiesService
	//addresses       *compute.GlobalAddressesService
	//forwardingrules *compute.GlobalForwardingRulesService
	//firewalls       *compute.FirewallsService
	//routers         *compute.RoutersService
}

// NewService returns a new service given the gcp api client.
func NewService(scope *scope.GKEClusterScope) *Service {
	return &Service{
		scope:           scope,
		project: scope.Container.Projects,
		//instances:       scope.Compute.Instances,
		//instancegroups:  scope.Compute.InstanceGroups,
		//networks:        scope.Compute.Networks,
		//subnetworks:     scope.Compute.Subnetworks,
		//healthchecks:    scope.Compute.HealthChecks,
		//backendservices: scope.Compute.BackendServices,
		//targetproxies:   scope.Compute.TargetTcpProxies,
		//addresses:       scope.Compute.GlobalAddresses,
		//forwardingrules: scope.Compute.GlobalForwardingRules,
		//firewalls:       scope.Compute.Firewalls,
		//routers:         scope.Compute.Routers,
	}
}
