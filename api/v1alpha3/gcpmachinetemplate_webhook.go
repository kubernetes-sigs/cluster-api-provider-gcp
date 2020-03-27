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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *GCPMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-gcpmachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinetemplates,versions=v1alpha3,name=validation.gcpmachinetemplate.infrastructure.x-k8s.io

var _ webhook.Validator = &GCPMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GCPMachineTemplate) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GCPMachineTemplate) ValidateUpdate(old runtime.Object) error {
	oldGCPMachineTemplate := old.(*GCPMachineTemplate)
	if !reflect.DeepEqual(r.Spec, oldGCPMachineTemplate.Spec) {
		return errors.New("gcpMachineTemplateSpec is immutable")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GCPMachineTemplate) ValidateDelete() error {
	return nil
}
