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

package scope

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedClusterScopeParams defines the input parameters used to create a new Scope.
type ManagedClusterScopeParams struct {
	GCPServices
	Client                 client.Client
	Cluster                *clusterv1.Cluster
	GCPManagedCluster      *infrav1exp.GCPManagedCluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
}

// NewManagedClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedClusterScope(ctx context.Context, params ManagedClusterScopeParams) (*ManagedClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GCPManagedCluster == nil {
		return nil, errors.New("failed to generate new scope from nil GCPManagedCluster")
	}

	if params.GCPServices.Compute == nil {
		computeSvc, err := newComputeService(ctx, params.GCPManagedCluster.Spec.CredentialsRef, params.Client)
		if err != nil {
			return nil, errors.Errorf("failed to create gcp compute client: %v", err)
		}

		params.GCPServices.Compute = computeSvc
	}

	helper, err := patch.NewHelper(params.GCPManagedCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedClusterScope{
		client:                 params.Client,
		Cluster:                params.Cluster,
		GCPManagedCluster:      params.GCPManagedCluster,
		GCPManagedControlPlane: params.GCPManagedControlPlane,
		GCPServices:            params.GCPServices,
		patchHelper:            helper,
	}, nil
}

// ManagedClusterScope defines the basic context for an actuator to operate upon.
type ManagedClusterScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster                *clusterv1.Cluster
	GCPManagedCluster      *infrav1exp.GCPManagedCluster
	GCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	GCPServices
}

// ANCHOR: ClusterGetter

// Cloud returns initialized cloud.
func (s *ManagedClusterScope) Cloud() cloud.Cloud {
	return newCloud(s.Project(), s.GCPServices)
}

// NetworkCloud returns initialized cloud.
func (s *ManagedClusterScope) NetworkCloud() cloud.Cloud {
	return newCloud(s.NetworkProject(), s.GCPServices)
}

// Project returns the current project name.
func (s *ManagedClusterScope) Project() string {
	return s.GCPManagedCluster.Spec.Project
}

// Region returns the cluster region.
func (s *ManagedClusterScope) Region() string {
	return s.GCPManagedCluster.Spec.Region
}

// Name returns the cluster name.
func (s *ManagedClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ManagedClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// NetworkName returns the cluster network unique identifier.
func (s *ManagedClusterScope) NetworkName() string {
	return pointer.StringDeref(s.GCPManagedCluster.Spec.Network.Name, "default")
}

// NetworkProject returns the cluster network unique identifier.
func (s *ManagedClusterScope) NetworkProject() string {
	return pointer.StringDeref(s.GCPManagedCluster.Spec.Network.HostProject, s.GCPManagedCluster.Spec.Project)
}

// IsSharedVpc returns the cluster network unique identifier.
func (s *ManagedClusterScope) IsSharedVpc() bool {
	if s.NetworkProject() != s.Project() {
		return true
	}
	return false
}

// NetworkLink returns the partial URL for the network.
func (s *ManagedClusterScope) NetworkLink() string {
	return fmt.Sprintf("projects/%s/global/networks/%s", s.NetworkProject(), s.NetworkName())
}

// Network returns the cluster network object.
func (s *ManagedClusterScope) Network() *infrav1.Network {
	return &s.GCPManagedCluster.Status.Network
}

// AdditionalLabels returns the cluster additional labels.
func (s *ManagedClusterScope) AdditionalLabels() infrav1.Labels {
	return s.GCPManagedCluster.Spec.AdditionalLabels
}

// ControlPlaneEndpoint returns the cluster control-plane endpoint.
func (s *ManagedClusterScope) ControlPlaneEndpoint() clusterv1.APIEndpoint {
	endpoint := s.GCPManagedCluster.Spec.ControlPlaneEndpoint
	endpoint.Port = pointer.Int32Deref(s.Cluster.Spec.ClusterNetwork.APIServerPort, 443)
	return endpoint
}

// FailureDomains returns the cluster failure domains.
func (s *ManagedClusterScope) FailureDomains() clusterv1.FailureDomains {
	return s.GCPManagedCluster.Status.FailureDomains
}

// ANCHOR_END: ClusterGetter

// ANCHOR: ClusterSetter

// SetReady sets cluster ready status.
func (s *ManagedClusterScope) SetReady() {
	s.GCPManagedCluster.Status.Ready = true
}

// SetFailureDomains sets cluster failure domains.
func (s *ManagedClusterScope) SetFailureDomains(fd clusterv1.FailureDomains) {
	s.GCPManagedCluster.Status.FailureDomains = fd
}

// SetControlPlaneEndpoint sets cluster control-plane endpoint.
func (s *ManagedClusterScope) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	s.GCPManagedCluster.Spec.ControlPlaneEndpoint = endpoint
}

// ANCHOR_END: ClusterSetter

// ANCHOR: ClusterNetworkSpec

// NetworkSpec returns google compute network spec.
func (s *ManagedClusterScope) NetworkSpec() *compute.Network {
	createSubnet := pointer.BoolDeref(s.GCPManagedCluster.Spec.Network.AutoCreateSubnetworks, true)
	network := &compute.Network{
		Name:                  s.NetworkName(),
		Description:           infrav1.ClusterTagKey(s.Name()),
		AutoCreateSubnetworks: createSubnet,
		ForceSendFields:       []string{"AutoCreateSubnetworks"},
	}

	return network
}

// NatRouterSpec returns google compute nat router spec.
func (s *ManagedClusterScope) NatRouterSpec() *compute.Router {
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

// SubnetSpecs returns google compute subnets spec.
func (s *ManagedClusterScope) SubnetSpecs() []*compute.Subnetwork {
	subnets := []*compute.Subnetwork{}
	for _, subnetwork := range s.GCPManagedCluster.Spec.Network.Subnets {
		secondaryIPRanges := []*compute.SubnetworkSecondaryRange{}
		for _, secondaryCidrBlock := range subnetwork.SecondaryCidrBlocks {
			secondaryIPRanges = append(secondaryIPRanges, &compute.SubnetworkSecondaryRange{IpCidrRange: secondaryCidrBlock})
		}
		subnets = append(subnets, &compute.Subnetwork{
			Name:                  subnetwork.Name,
			Region:                subnetwork.Region,
			EnableFlowLogs:        pointer.BoolDeref(subnetwork.EnableFlowLogs, false),
			PrivateIpGoogleAccess: pointer.BoolDeref(subnetwork.PrivateGoogleAccess, false),
			IpCidrRange:           subnetwork.CidrBlock,
			SecondaryIpRanges:     secondaryIPRanges,
			Description:           pointer.StringDeref(subnetwork.Description, infrav1.ClusterTagKey(s.Name())),
			Network:               s.NetworkLink(),
			Purpose:               pointer.StringDeref(subnetwork.Purpose, "PRIVATE_RFC_1918"),
			Role:                  "ACTIVE",
		})
	}

	return subnets
}

// ANCHOR: ClusterFirewallSpec

// FirewallRulesSpec returns google compute firewall spec.
func (s *ManagedClusterScope) FirewallRulesSpec() []*compute.Firewall {
	firewallRules := []*compute.Firewall{
		{
			Name:    fmt.Sprintf("allow-%s-healthchecks", s.Name()),
			Network: s.NetworkLink(),
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
			Network: s.NetworkLink(),
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

// PatchObject persists the cluster configuration and status.
func (s *ManagedClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.GCPManagedCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ManagedClusterScope) Close() error {
	return s.PatchObject()
}
