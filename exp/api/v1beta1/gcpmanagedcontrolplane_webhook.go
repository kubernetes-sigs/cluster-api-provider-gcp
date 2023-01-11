/*
Copyright 2022 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var gcpmanagedcontrolplanelog = logf.Log.WithName("gcpmanagedcontrolplane-resource")

func (r *GCPManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=mgcpmanagedcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GCPManagedControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) Default() {
	gcpmanagedcontrolplanelog.Info("default", "name", r.Name)
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=vgcpmanagedcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GCPManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateCreate() error {
	gcpmanagedcontrolplanelog.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateUpdate(old runtime.Object) error {
	gcpmanagedcontrolplanelog.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateDelete() error {
	gcpmanagedcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}
