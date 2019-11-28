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

package v1alpha3

import (
	"errors"
	"reflect"

	runtime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager sets up the webhook with the manager.
func (m *GCPMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-gcpmachine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,versions=v1alpha3,name=validation.gcpmachine.infrastructure.x-k8s.io

var _ webhook.Validator = &GCPMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (m *GCPMachine) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (m *GCPMachine) ValidateUpdate(old runtime.Object) error {
	return m.validate(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (m *GCPMachine) ValidateDelete() error {
	return nil
}

func (m *GCPMachine) validate(old runtime.Object) error {
	oldGCPMachine := old.(*GCPMachine)
	if !reflect.DeepEqual(m.Spec, oldGCPMachine.Spec) {
		return errors.New("gcpMachineSpec is immutable")
	}

	return nil
}
