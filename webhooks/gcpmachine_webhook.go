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

package webhooks

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (m *GCPMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.GCPMachine{}).
		WithValidator(m).
		WithDefaulter(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,versions=v1beta1,name=validation.gcpmachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmachine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpmachines,versions=v1beta1,name=default.gcpmachine.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// GCPMachine implements a validating and defaulting webhook for GCPMachine.
type GCPMachine struct{}

var (
	_ webhook.CustomValidator = &GCPMachine{}
	_ webhook.CustomDefaulter = &GCPMachine{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*GCPMachine) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, ok := obj.(*infrav1.GCPMachine)
	if !ok {
		return nil, fmt.Errorf("expected an GCPMachine object but got %T", m)
	}

	clusterlog.Info("validate create", "name", m.Name)

	if err := ValidateConfidentialCompute(m.Spec.ConfidentialCompute, m.Spec.OnHostMaintenance, m.Spec.InstanceType); err != nil {
		return nil, err
	}
	return nil, ValidateCustomerEncryptionKey(m.Spec.RootDiskEncryptionKey, m.Spec.AdditionalDisks)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*GCPMachine) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	m, ok := newObj.(*infrav1.GCPMachine)
	if !ok {
		return nil, fmt.Errorf("expected an GCPMachine object but got %T", m)
	}

	newGCPMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(m)
	if err != nil {
		return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
			field.InternalError(nil, errors.Wrap(err, "failed to convert new GCPMachine to unstructured object")),
		})
	}
	oldGCPMachine, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj)
	if err != nil {
		return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
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

	if !reflect.DeepEqual(oldGCPMachineSpec, newGCPMachineSpec) {
		return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("GCPMachine").GroupKind(), m.Name, field.ErrorList{
			field.Forbidden(field.NewPath("spec"), "cannot be modified"),
		})
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*GCPMachine) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Default implements webhookutil.defaulter so a webhook will be registered for the type.
func (*GCPMachine) Default(_ context.Context, _ runtime.Object) error {
	return nil
}


