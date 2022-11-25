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
var gcpmanagedmachinepoollog = logf.Log.WithName("gcpmanagedmachinepool-resource")

func (r *GCPManagedMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedmachinepool,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedmachinepools,verbs=create;update,versions=v1beta1,name=mgcpmanagedmachinepool.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GCPManagedMachinePool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *GCPManagedMachinePool) Default() {
	gcpmanagedmachinepoollog.Info("default", "name", r.Name)
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedmachinepool,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedmachinepools,verbs=create;update,versions=v1beta1,name=vgcpmanagedmachinepool.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GCPManagedMachinePool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedMachinePool) ValidateCreate() error {
	gcpmanagedmachinepoollog.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedMachinePool) ValidateUpdate(old runtime.Object) error {
	gcpmanagedmachinepoollog.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedMachinePool) ValidateDelete() error {
	gcpmanagedmachinepoollog.Info("validate delete", "name", r.Name)

	return nil
}
