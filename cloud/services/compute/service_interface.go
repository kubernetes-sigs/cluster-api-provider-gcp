/*
Copyright 2019 The Kubernetes Authors.

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
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
)

type ServiceInterface interface {
	ReconcileNetwork() error
	DeleteNetwork() error
	ReconcileLoadbalancers() error
	UpdateBackendServices() error
	DeleteLoadbalancers() error
	InstanceIfExists(scope *scope.MachineScope) (*compute.Instance, error)
	CreateInstance(scope *scope.MachineScope) (*compute.Instance, error)
	TerminateInstanceAndWait(scope *scope.MachineScope) error
	ReconcileInstanceGroups() error
	DeleteInstanceGroups() error
	GetOrCreateInstanceGroup(zone, name string) (*compute.InstanceGroup, error)
	GetInstanceGroupMembers(zone, name string) ([]*compute.InstanceWithNamedPorts, error)
	EnsureInstanceGroupMember(zone, name string, i *compute.Instance) error
	ReconcileFirewalls() error
	DeleteFirewalls() error
}
