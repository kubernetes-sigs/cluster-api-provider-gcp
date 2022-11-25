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
var gcpmanagedclusterlog = logf.Log.WithName("gcpmanagedcluster-resource")

func (r *GCPManagedCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters,verbs=create;update,versions=v1beta1,name=mgcpmanagedcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GCPManagedCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *GCPManagedCluster) Default() {
	gcpmanagedclusterlog.Info("default", "name", r.Name)
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedclusters,verbs=create;update,versions=v1beta1,name=vgcpmanagedcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GCPManagedCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedCluster) ValidateCreate() error {
	gcpmanagedclusterlog.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedCluster) ValidateUpdate(old runtime.Object) error {
	gcpmanagedclusterlog.Info("validate update", "name", r.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedCluster) ValidateDelete() error {
	gcpmanagedclusterlog.Info("validate delete", "name", r.Name)

	return nil
}
