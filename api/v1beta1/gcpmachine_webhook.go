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

package v1beta1

import (
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var _ = logf.Log.WithName("gcpmachine-resource")

func (m *GCPMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,versions=v1beta1,name=validation.gcpmachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,versions=v1beta1,name=default.gcpmachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Validator = &GCPMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *GCPMachine) ValidateCreate() error {
	clusterlog.Info("validate create", "name", m.Name)

	if m.Spec.AcceleratorConfigs != nil {
		if m.Spec.OnHostMaintenance == "MIGRATE" {
			return errors.New("OnHostMaintenance cannot be set to MIGRATE when AcceleratorConfig is set")
		}
		if !m.Spec.AutomaticRestart {
			return errors.New("AutomaticRestart cannot be set to false when AcceleratorConfig is set")
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *GCPMachine) ValidateUpdate(old runtime.Object) error {
	newGCPMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(m)
	if err != nil {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
			field.InternalError(nil, errors.Wrap(err, "failed to convert new GCPMachine to unstructured object")),
		})
	}
	oldGCPMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
			field.InternalError(nil, errors.Wrap(err, "failed to convert old GCPMachine to unstructured object")),
		})
	}

	newGCPMachineSpec := newGCPMachine["spec"].(map[string]interface{})
	oldGCPMachineSpec := oldGCPMachine["spec"].(map[string]interface{})

	// allow changes to providerID
	delete(oldGCPMachineSpec, "providerID")
	delete(newGCPMachineSpec, "providerID")

	// allow changes to additionalLabels
	delete(oldGCPMachineSpec, "additionalLabels")
	delete(newGCPMachineSpec, "additionalLabels")

	// allow changes to additionalNetworkTags
	delete(oldGCPMachineSpec, "additionalNetworkTags")
	delete(newGCPMachineSpec, "additionalNetworkTags")

	// allow changes to OnHostMaintenance
	delete(oldGCPMachineSpec, "onHostMaintenance")
	delete(newGCPMachineSpec, "onHostMaintenance")

	// allow changes to AutomaticRestart
	delete(oldGCPMachineSpec, "automaticRestart")
	delete(newGCPMachineSpec, "automaticRestart")

	if !reflect.DeepEqual(oldGCPMachineSpec, newGCPMachineSpec) {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
			field.Forbidden(field.NewPath("spec"), "cannot be modified"),
		})
	}

	errs := field.ErrorList{}
	if !reflect.DeepEqual(oldGCPMachineSpec, newGCPMachineSpec) {
		errs = append(errs, field.Invalid(field.NewPath("spec", "AcceleratorConfig"),
			m.Spec.AcceleratorConfigs, "cannot be modified"),
		)
	}

	if len(errs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *GCPMachine) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", m.Name)

	return nil
}

// Default implements webhookutil.defaulter so a webhook will be registered for the type.
func (m *GCPMachine) Default() {
	clusterlog.Info("default", "name", m.Name)
}
