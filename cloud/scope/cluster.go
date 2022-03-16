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

package scope

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	GCPServices
	Client     client.Client
	Cluster    *clusterv1.Cluster
	GCPCluster *infrav1.GCPCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GCPCluster == nil {
		return nil, errors.New("failed to generate new scope from nil GCPCluster")
	}

	computeSvc, err := compute.NewService(context.TODO())
	if err != nil {
		return nil, errors.Errorf("failed to create gcp compute client: %v", err)
	}

	if params.GCPServices.Compute == nil {
		params.GCPServices.Compute = computeSvc
	}

	helper, err := patch.NewHelper(params.GCPCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		GCPCluster:  params.GCPCluster,
		GCPServices: params.GCPServices,
		patchHelper: helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	GCPCluster *infrav1.GCPCluster
	GCPServices
}

// ANCHOR: ClusterGetter

// Cloud returns initialized cloud.
func (s *ClusterScope) Cloud() cloud.Cloud {
	return newCloud(s.Project(), s.GCPServices)
}

// Project returns the current project name.
func (s *ClusterScope) Project() string {
	return s.GCPCluster.Spec.Project
}

// Region returns the cluster region.
func (s *ClusterScope) Region() string {
	return s.GCPCluster.Spec.Region
}

// Name returns the cluster name.
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// NetworkName returns the cluster network unique identifier.
func (s *ClusterScope) NetworkName() string {
	return pointer.StringDeref(s.GCPCluster.Spec.Network.Name, "default")
}

// Network returns the cluster network object.
func (s *ClusterScope) Network() *infrav1.Network {
	return &s.GCPCluster.Status.Network
}

// AdditionalLabels returns the cluster additional labels.
func (s *ClusterScope) AdditionalLabels() infrav1.Labels {
	return s.GCPCluster.Spec.AdditionalLabels
}

// ControlPlaneEndpoint returns the cluster control-plane endpoint.
func (s *ClusterScope) ControlPlaneEndpoint() clusterv1.APIEndpoint {
	endpoint := s.GCPCluster.Spec.ControlPlaneEndpoint
	endpoint.Port = pointer.Int32Deref(s.Cluster.Spec.ClusterNetwork.APIServerPort, 443)
	return endpoint
}

// FailureDomains returns the cluster failure domains.
func (s *ClusterScope) FailureDomains() clusterv1.FailureDomains {
	return s.GCPCluster.Status.FailureDomains
}

// ANCHOR_END: ClusterGetter

// ANCHOR: ClusterSetter

// SetReady sets cluster ready status.
func (s *ClusterScope) SetReady() {
	s.GCPCluster.Status.Ready = true
}

// SetFailureDomains sets cluster failure domains.
func (s *ClusterScope) SetFailureDomains(fd clusterv1.FailureDomains) {
	s.GCPCluster.Status.FailureDomains = fd
}

// SetControlPlaneEndpoint sets cluster control-plane endpoint.
func (s *ClusterScope) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	s.GCPCluster.Spec.ControlPlaneEndpoint = endpoint
}

// ANCHOR_END: ClusterSetter

// ANCHOR: ClusterNetworkSpec

// NetworkSpec returns google compute network spec.
func (s *ClusterScope) NetworkSpec() *compute.Network {
	createSubnet := pointer.BoolDeref(s.GCPCluster.Spec.Network.AutoCreateSubnetworks, true)
	network := &compute.Network{
		Name:                  s.NetworkName(),
		Description:           infrav1.ClusterTagKey(s.Name()),
		AutoCreateSubnetworks: createSubnet,
	}

	return network
}

// NatRouterSpec returns google compute nat router spec.
func (s *ClusterScope) NatRouterSpec() *compute.Router {
	networkSpec := s.NetworkSpec()
	return &compute.Router{
		Name: fmt.Sprintf("%s-%s", networkSpec.Name, "router"),
		Nats: []*compute.RouterNat{
			{
				Name:                          fmt.Sprintf("%s-%s", networkSpec.Name, "nat"),
				NatIpAllocateOption:           "AUTO_ONLY",
				SourceSubnetworkIpRangesToNat: "ALL_SUBNETWORKS_ALL_IP_RANGES",
			},
		},
	}
}

// ANCHOR_END: ClusterNetworkSpec

// ANCHOR: ClusterFirewallSpec

// FirewallRulesSpec returns google compute firewall spec.
func (s *ClusterScope) FirewallRulesSpec() []*compute.Firewall {
	network := s.Network()
	firewallRules := []*compute.Firewall{
		{
			Name:    fmt.Sprintf("allow-%s-healthchecks", s.Name()),
			Network: *network.SelfLink,
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: "TCP",
					Ports: []string{
						strconv.FormatInt(6443, 10),
					},
				},
			},
			Direction: "INGRESS",
			SourceRanges: []string{
				"35.191.0.0/16",
				"130.211.0.0/22",
			},
			TargetTags: []string{
				fmt.Sprintf("%s-control-plane", s.Name()),
			},
		},
		{
			Name:    fmt.Sprintf("allow-%s-cluster", s.Name()),
			Network: *network.SelfLink,
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: "all",
				},
			},
			Direction: "INGRESS",
			SourceTags: []string{
				fmt.Sprintf("%s-control-plane", s.Name()),
				fmt.Sprintf("%s-node", s.Name()),
			},
			TargetTags: []string{
				fmt.Sprintf("%s-control-plane", s.Name()),
				fmt.Sprintf("%s-node", s.Name()),
			},
		},
	}

	return firewallRules
}

// ANCHOR_END: ClusterFirewallSpec

// ANCHOR: ClusterControlPlaneSpec

// AddressSpec returns google compute address spec.
func (s *ClusterScope) AddressSpec() *compute.Address {
	return &compute.Address{
		Name:        fmt.Sprintf("%s-%s", s.Name(), infrav1.APIServerRoleTagValue),
		AddressType: "EXTERNAL",
		IpVersion:   "IPV4",
	}
}

// BackendServiceSpec returns google compute backend-service spec.
func (s *ClusterScope) BackendServiceSpec() *compute.BackendService {
	return &compute.BackendService{
		Name:                fmt.Sprintf("%s-%s", s.Name(), infrav1.APIServerRoleTagValue),
		LoadBalancingScheme: "EXTERNAL",
		PortName:            "apiserver",
		Protocol:            "TCP",
		TimeoutSec:          int64((10 * time.Minute).Seconds()),
	}
}

// ForwardingRuleSpec returns google compute forwarding-rule spec.
func (s *ClusterScope) ForwardingRuleSpec() *compute.ForwardingRule {
	port := pointer.Int32Deref(s.Cluster.Spec.ClusterNetwork.APIServerPort, 443)
	portRange := fmt.Sprintf("%d-%d", port, port)
	return &compute.ForwardingRule{
		Name:                fmt.Sprintf("%s-%s", s.Name(), infrav1.APIServerRoleTagValue),
		IPProtocol:          "TCP",
		LoadBalancingScheme: "EXTERNAL",
		PortRange:           portRange,
	}
}

// HealthCheckSpec returns google compute health-check spec.
func (s *ClusterScope) HealthCheckSpec() *compute.HealthCheck {
	return &compute.HealthCheck{
		Name: fmt.Sprintf("%s-%s", s.Name(), infrav1.APIServerRoleTagValue),
		Type: "SSL",
		SslHealthCheck: &compute.SSLHealthCheck{
			Port:              6443,
			PortSpecification: "USE_FIXED_PORT",
		},
		CheckIntervalSec:   10,
		TimeoutSec:         5,
		HealthyThreshold:   5,
		UnhealthyThreshold: 3,
	}
}

// InstanceGroupSpec returns google compute instance-group spec.
func (s *ClusterScope) InstanceGroupSpec(zone string) *compute.InstanceGroup {
	port := pointer.Int32Deref(s.GCPCluster.Spec.Network.LoadBalancerBackendPort, 6443)
	return &compute.InstanceGroup{
		Name: fmt.Sprintf("%s-%s-%s", s.Name(), infrav1.APIServerRoleTagValue, zone),
		NamedPorts: []*compute.NamedPort{
			{
				Name: "apiserver",
				Port: int64(port),
			},
		},
	}
}

// TargetTCPProxySpec returns google compute target-tcp-proxy spec.
func (s *ClusterScope) TargetTCPProxySpec() *compute.TargetTcpProxy {
	return &compute.TargetTcpProxy{
		Name:        fmt.Sprintf("%s-%s", s.Name(), infrav1.APIServerRoleTagValue),
		ProxyHeader: "NONE",
	}
}

// ANCHOR_END: ClusterControlPlaneSpec

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.GCPCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.PatchObject()
}
