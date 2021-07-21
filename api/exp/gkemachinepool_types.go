package exp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GKEMachinePoolSpec struct {

	Name string `json:"name"`

	// Number of replicas
	Number int32 `json:"number"`

	// MachineType is the type of instance to create. Example: n1.standard-2
	MachineType string `json:"machineType"`

}

type GKEMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

}

func (G GKEMachinePoolStatus) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func (G GKEMachinePoolStatus) DeepCopy() *GKEMachinePoolStatus {
	panic("implement me")
}

func (in *GKEMachinePoolStatus) DeepCopyInto(out *GKEMachinePoolStatus) {
	panic("implement me")
}


type GKEMachinePool struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKEMachinePoolSpec   `json:"spec,omitempty"`
    Status GKEMachinePoolStatus `json:"status,omitempty"`
}

func (G GKEMachinePool) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (G GKEMachinePool) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func (G GKEMachinePool) DeepCopy() *GKEMachinePool {
	panic("implement me")
}

func (in *GKEMachinePool) DeepCopyInto(out *GKEMachinePool) {
	panic("implement me")
}

