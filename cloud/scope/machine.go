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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha4"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	GCPClients
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	Machine    *clusterv1.Machine
	GCPCluster *infrav1.GCPCluster
	GCPMachine *infrav1.GCPMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachineScope")
	}
	if params.GCPCluster == nil {
		return nil, errors.New("gcp cluster is required when creating a MachineScope")
	}
	if params.GCPMachine == nil {
		return nil, errors.New("gcp machine is required when creating a MachineScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.GCPMachine, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &MachineScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		Machine:     params.Machine,
		GCPCluster:  params.GCPCluster,
		GCPMachine:  params.GCPMachine,
		Logger:      params.Logger,
		patchHelper: helper,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	Machine    *clusterv1.Machine
	GCPCluster *infrav1.GCPCluster
	GCPMachine *infrav1.GCPMachine
}

// Region returns the GCPMachine region.
func (m *MachineScope) Region() string {
	return m.GCPCluster.Spec.Region
}

// Zone returns the FailureDomain for the GCPMachine.
func (m *MachineScope) Zone() string {
	if m.Machine.Spec.FailureDomain == nil {
		return ""
	}

	return *m.Machine.Spec.FailureDomain
}

// Name returns the GCPMachine name.
func (m *MachineScope) Name() string {
	return m.GCPMachine.Name
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.GCPMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return "control-plane"
	}

	return "node"
}

// GetInstanceID returns the GCPMachine instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetInstanceID() *string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return nil
	}

	return pointer.StringPtr(parsed.ID())
}

// GetProviderID returns the GCPMachine providerID from the spec.
func (m *MachineScope) GetProviderID() string {
	if m.GCPMachine.Spec.ProviderID != nil {
		return *m.GCPMachine.Spec.ProviderID
	}

	return ""
}

// SetProviderID sets the GCPMachine providerID in spec.
func (m *MachineScope) SetProviderID(v string) {
	m.GCPMachine.Spec.ProviderID = pointer.StringPtr(v)
}

// GetInstanceStatus returns the GCPMachine instance status.
func (m *MachineScope) GetInstanceStatus() *infrav1.InstanceStatus {
	return m.GCPMachine.Status.InstanceStatus
}

// SetInstanceStatus sets the GCPMachine instance status.
func (m *MachineScope) SetInstanceStatus(v infrav1.InstanceStatus) {
	m.GCPMachine.Status.InstanceStatus = &v
}

// SetReady sets the GCPMachine Ready Status.
func (m *MachineScope) SetReady() {
	m.GCPMachine.Status.Ready = true
}

// SetFailureMessage sets the GCPMachine status failure message.
func (m *MachineScope) SetFailureMessage(v error) {
	m.GCPMachine.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the GCPMachine status failure reason.
func (m *MachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.GCPMachine.Status.FailureReason = &v
}

// SetAnnotation sets a key value annotation on the GCPMachine.
func (m *MachineScope) SetAnnotation(key, value string) {
	if m.GCPMachine.Annotations == nil {
		m.GCPMachine.Annotations = map[string]string{}
	}
	m.GCPMachine.Annotations[key] = value
}

// SetAddresses sets the addresses field on the GCPMachine.
func (m *MachineScope) SetAddresses(addressList []corev1.NodeAddress) {
	m.GCPMachine.Status.Addresses = addressList
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData() (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for GCPMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return string(value), nil
}

// PatchObject persists the cluster configuration and status.
func (m *MachineScope) PatchObject() error {
	return m.patchHelper.Patch(context.TODO(), m.GCPMachine)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachineScope) Close() error {
	return m.PatchObject()
}
