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

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinetemplates,versions=v1beta1,name=validation.gcpmachinetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachinetemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinetemplates,versions=v1beta1,name=default.gcpmachinetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// log is for logging in this package.
var _ = logf.Log.WithName("gcpmachinetemplate-resource")

func (r *GCPMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Validator = &GCPMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPMachineTemplate) ValidateCreate() error {
	clusterlog.Info("validate create", "name", r.Name)

	return validateConfidentialCompute(r.Spec.Template.Spec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPMachineTemplate) ValidateUpdate(old runtime.Object) error {
	newGCPMachineTemplate, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachineTemplate").GroupKind(), r.Name, field.ErrorList{
			field.InternalError(nil, errors.Wrap(err, "failed to convert new GCPMachineTemplate to unstructured object")),
		})
	}
	oldGCPMachineTemplate, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachineTemplate").GroupKind(), r.Name, field.ErrorList{
			field.InternalError(nil, errors.Wrap(err, "failed to convert old GCPMachineTemplate to unstructured object")),
		})
	}

	newGCPMachineTemplateSpec := newGCPMachineTemplate["spec"].(map[string]interface{})
	oldGCPMachineTemplateSpec := oldGCPMachineTemplate["spec"].(map[string]interface{})

	newGCPMachineTemplateSpecTemplate := newGCPMachineTemplateSpec["template"].(map[string]interface{})
	oldGCPMachineTemplateSpecTemplate := oldGCPMachineTemplateSpec["template"].(map[string]interface{})

	newGCPMachineTemplateSpecTemplateSpec := newGCPMachineTemplateSpecTemplate["spec"].(map[string]interface{})
	oldGCPMachineTemplateSpecTemplateSpec := oldGCPMachineTemplateSpecTemplate["spec"].(map[string]interface{})
	// allow changes to providerID
	delete(oldGCPMachineTemplateSpecTemplateSpec, "providerID")
	delete(newGCPMachineTemplateSpecTemplateSpec, "providerID")

	// allow changes to additionalLabels
	delete(oldGCPMachineTemplateSpecTemplateSpec, "additionalLabels")
	delete(newGCPMachineTemplateSpecTemplateSpec, "additionalLabels")

	// allow changes to additionalNetworkTags
	delete(oldGCPMachineTemplateSpecTemplateSpec, "additionalNetworkTags")
	delete(newGCPMachineTemplateSpecTemplateSpec, "additionalNetworkTags")

	delete(newGCPMachineTemplateSpecTemplateSpec, "ipForwarding")

	if !reflect.DeepEqual(oldGCPMachineTemplateSpecTemplateSpec, newGCPMachineTemplateSpecTemplateSpec) {
		return apierrors.NewInvalid(GroupVersion.WithKind("GCPMachineTemplate").GroupKind(), r.Name, field.ErrorList{
			field.Forbidden(field.NewPath("spec"), "cannot be modified"),
		})
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPMachineTemplate) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", r.Name)

	return nil
}

// Default implements webhookutil.defaulter so a webhook will be registered for the type.
func (r *GCPMachineTemplate) Default() {
	clusterlog.Info("default", "name", r.Name)
}
