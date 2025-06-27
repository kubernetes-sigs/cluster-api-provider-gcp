/*
Copyright 2025 The Kubernetes Authors.

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
	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var gmcptlog = logf.Log.WithName("gcpmanagedcontrolplane-resource")

func (r *GCPManagedControlPlaneTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcontrolplanetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedcontrolplanetemplates,verbs=create;update,versions=v1beta1,name=vgcpmanagedcontrolplanetemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GCPManagedControlPlaneTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlaneTemplate) ValidateCreate() (admission.Warnings, error) {
	gmcptlog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList
	var allWarns admission.Warnings

	if r.Spec.Template.Spec.EnableAutopilot && r.Spec.Template.Spec.ReleaseChannel == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "ReleaseChannel"), "Release channel is required for an autopilot enabled cluster"))
	}

	if r.Spec.Template.Spec.EnableAutopilot && r.Spec.Template.Spec.LoggingService != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "LoggingService"),
			r.Spec.Template.Spec.LoggingService, "can't be set when autopilot is enabled"))
	}

	if r.Spec.Template.Spec.EnableAutopilot && r.Spec.Template.Spec.MonitoringService != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "MonitoringService"),
			r.Spec.Template.Spec.LoggingService, "can't be set when autopilot is enabled"))
	}

	if len(allErrs) == 0 {
		return allWarns, nil
	}

	return allWarns, apierrors.NewInvalid(GroupVersion.WithKind("GCPManagedControlPlaneTemplate").GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlaneTemplate) ValidateUpdate(oldRaw runtime.Object) (admission.Warnings, error) {
	gmcptlog.Info("validate update", "name", r.Name)
	var allErrs field.ErrorList
	old := oldRaw.(*GCPManagedControlPlaneTemplate)

	if !cmp.Equal(r.Spec.Template.Spec.Project, old.Spec.Template.Spec.Project) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Project"),
				r.Spec.Template.Spec.Project, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.Template.Spec.Location, old.Spec.Template.Spec.Location) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Location"),
				r.Spec.Template.Spec.Location, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.Template.Spec.EnableAutopilot, old.Spec.Template.Spec.EnableAutopilot) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "EnableAutopilot"),
				r.Spec.Template.Spec.EnableAutopilot, "field is immutable"),
		)
	}

	if old.Spec.Template.Spec.EnableAutopilot && r.Spec.Template.Spec.LoggingService != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "LoggingService"),
			r.Spec.Template.Spec.LoggingService, "can't be set when autopilot is enabled"))
	}

	if old.Spec.Template.Spec.EnableAutopilot && r.Spec.Template.Spec.MonitoringService != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "MonitoringService"),
			r.Spec.Template.Spec.LoggingService, "can't be set when autopilot is enabled"))
	}

	if r.Spec.Template.Spec.LoggingService != nil {
		err := r.Spec.Template.Spec.LoggingService.Validate()
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "LoggingService"),
				r.Spec.Template.Spec.LoggingService, err.Error()))
		}
	}

	if r.Spec.Template.Spec.MonitoringService != nil {
		err := r.Spec.Template.Spec.MonitoringService.Validate()
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "MonitoringService"),
				r.Spec.Template.Spec.MonitoringService, err.Error()))
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(GroupVersion.WithKind("GCPManagedControlPlaneTemplate").GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlaneTemplate) ValidateDelete() (admission.Warnings, error) {
	gmcptlog.Info("validate delete", "name", r.Name)

	return nil, nil
}
