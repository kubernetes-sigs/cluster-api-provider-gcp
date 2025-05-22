/*
Copyright The Kubernetes Authors.
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// GCPMachinePoolMachineFinalizer indicates the GCPMachinePoolMachine name the GCPMachinePoolMachine belongs.
	GCPMachinePoolMachineFinalizer = "gcpmachinepoolmachine.infrastructure.cluster.x-k8s.io"
)

// GCPMachinePoolMachineSpec defines the desired state of GCPMachinePoolMachine and the GCP instances that it will create.
type GCPMachinePoolMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// InstanceID is the unique identifier for the instance in the cloud provider.
	// +optional
	InstanceID string `json:"instanceID,omitempty"`
}

// GCPMachinePoolMachineStatus defines the observed state of GCPMachinePoolMachine and the GCP instances that it manages.
type GCPMachinePoolMachineStatus struct {

	// NodeRef will point to the corresponding Node if it exists.
	// +optional
	NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// Version defines the Kubernetes version for the VM Instance
	// +optional
	Version string `json:"version,omitempty"`

	// InstanceName is the name of the Machine Instance within the VMSS
	// +optional
	InstanceName string `json:"instanceName,omitempty"`

	// LatestModelApplied is true when the latest instance template has been applied to the machine.
	// +optional
	LatestModelApplied bool `json:"latestModelApplied,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// LastOperation is a string that contains the last operation that was performed on the machine.
	// +optional
	LastOperation string `json:"lastOperation,omitempty"`

	// ProvisioningState is the state of the machine pool instance.
	ProvisioningState ProvisioningState `json:"provisioningState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the MachinePool and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the MachinePool's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of MachinePools
	// can be added as events to the MachinePool object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the MachinePool and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the MachinePool's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of MachinePools
	// can be added as events to the MachinePool object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions specifies the conditions for the managed machine pool
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="A machine pool machine belongs to a GCPMachinePool"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"

// GCPMachinePoolMachine is the Schema for the GCPMachinePoolMachines API and represents a GCP Machine Pool.
type GCPMachinePoolMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCPMachinePoolMachineSpec   `json:"spec,omitempty"`
	Status GCPMachinePoolMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GCPMachinePoolMachineList contains a list of GCPMachinePoolMachine resources.
type GCPMachinePoolMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCPMachinePoolMachine `json:"items"`
}

// GetConditions returns the conditions for the GCPManagedMachinePool.
func (r *GCPMachinePoolMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the status conditions for the GCPManagedMachinePool.
func (r *GCPMachinePoolMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
func init() {
	infrav1.SchemeBuilder.Register(&GCPMachinePoolMachine{}, &GCPMachinePoolMachineList{})
}
