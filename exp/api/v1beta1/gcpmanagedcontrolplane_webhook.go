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
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-gcp/util/hash"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	maxClusterNameLength = 40
	resourcePrefix       = "capg-"
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

	if r.Spec.ClusterName == "" {
		gcpmanagedcontrolplanelog.Info("ClusterName is empty, generating name")
		name, err := generateGKEName(r.Name, r.Namespace, maxClusterNameLength)
		if err != nil {
			gcpmanagedcontrolplanelog.Error(err, "failed to create GKE cluster name")
			return
		}

		gcpmanagedcontrolplanelog.Info("defaulting GKE cluster name", "cluster-name", name)
		r.Spec.ClusterName = name
	}

	// If EnableComponents has components specified, SystemComponents must be included as one of them
	if r.Spec.LoggingConfig != nil && len(r.Spec.LoggingConfig.EnableComponents) > 0 {
		containsSystemComponents := false
		for _, c := range r.Spec.LoggingConfig.EnableComponents {
			if c == SystemComponents {
				containsSystemComponents = true
			}
		}
		if !containsSystemComponents {
			r.Spec.LoggingConfig.EnableComponents = append(r.Spec.LoggingConfig.EnableComponents, SystemComponents)
		}
	}
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpmanagedcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=gcpmanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=vgcpmanagedcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GCPManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateCreate() (admission.Warnings, error) {
	gcpmanagedcontrolplanelog.Info("validate create", "name", r.Name)
	allErrs := ValidateGcpManagedControlPlane(r)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(GroupVersion.WithKind("GCPManagedControlPlane").GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateUpdate(oldRaw runtime.Object) (admission.Warnings, error) {
	gcpmanagedcontrolplanelog.Info("validate update", "name", r.Name)
	var allErrs field.ErrorList

	old := oldRaw.(*GCPManagedControlPlane)
	if !cmp.Equal(r.Spec.ClusterName, old.Spec.ClusterName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ClusterName"),
				r.Spec.ClusterName, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.Project, old.Spec.Project) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Project"),
				r.Spec.Project, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.Location, old.Spec.Location) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Location"),
				r.Spec.Location, "field is immutable"),
		)
	}

	// Mutability determined by what fields containerpb supports for update and GKE allows
	// https://pkg.go.dev/cloud.google.com/go/container/apiv1/containerpb

	if !cmp.Equal(r.Spec.ClusterIpv4Cidr, old.Spec.ClusterIpv4Cidr) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ClusterIpv4Cidr"),
				pointer.StringDeref(r.Spec.ClusterIpv4Cidr, ""), "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.EnableAutopilot, old.Spec.EnableAutopilot) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "EnableAutopilot"),
				r.Spec.EnableAutopilot, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.ResourceLabels, old.Spec.ResourceLabels) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ResourceLabels"),
				r.Spec.ResourceLabels, "field is immutable"),
		)
	}

	if !cmp.Equal(r.Spec.IPAllocationPolicy, old.Spec.IPAllocationPolicy) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "IPAllocationPolicy"),
				r.Spec.IPAllocationPolicy, "field is immutable"),
		)
	}

	// Normalize the nil values
	if r.Spec.PrivateClusterConfig == nil {
		r.Spec.PrivateClusterConfig = &PrivateClusterConfig{}
	}
	if old.Spec.PrivateClusterConfig == nil {
		old.Spec.PrivateClusterConfig = &PrivateClusterConfig{}
	}
	// PrivateClusterMasterGlobalAccessEnabled is the only field in PrivateClusterConfig that is mutable
	r.Spec.PrivateClusterConfig.PrivateClusterMasterGlobalAccessEnabled = old.Spec.PrivateClusterConfig.PrivateClusterMasterGlobalAccessEnabled
	if !cmp.Equal(r.Spec.PrivateClusterConfig, old.Spec.PrivateClusterConfig) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "PrivateClusterConfig"),
				r.Spec.IPAllocationPolicy, "field is immutable besides PrivateClusterMasterGlobalAccessEnabled"),
		)
	}

	if old.Spec.NetworkConfig != nil {
		if old.Spec.NetworkConfig.DNSConfig != nil && (r.Spec.NetworkConfig == nil || r.Spec.NetworkConfig.DNSConfig == nil) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "NetworkConfig", "DNSConfig"),
					r.Spec.NetworkConfig, "DNSConfig can not be removed after creation"),
			)
		}
		if r.Spec.NetworkConfig != nil && !cmp.Equal(r.Spec.NetworkConfig.DatapathProvider, old.Spec.NetworkConfig.DatapathProvider) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "NetworkConfig", "DatapathProvider"),
					r.Spec.NetworkConfig.DatapathProvider, "field is immutable"))
		}
	}

	if !cmp.Equal(r.Spec.DefaultMaxPodsConstraint, old.Spec.DefaultMaxPodsConstraint) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "DefaultMaxPodsConstraint"),
				r.Spec.DefaultMaxPodsConstraint, "field is immutable"),
		)
	}

	// Check that the updated spec is valid in general
	allErrs = append(allErrs, ValidateGcpManagedControlPlane(r)...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(GroupVersion.WithKind("GCPManagedControlPlane").GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *GCPManagedControlPlane) ValidateDelete() (admission.Warnings, error) {
	gcpmanagedcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil, nil
}

// ValidateGcpManagedControlPlane is a wrapper function for validating all fields for a given GCPManagedControlPlane.
func ValidateGcpManagedControlPlane(cp *GCPManagedControlPlane) field.ErrorList {
	var allErrs field.ErrorList

	if len(cp.Spec.ClusterName) > maxClusterNameLength {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ClusterName"),
				cp.Spec.ClusterName, fmt.Sprintf("cluster name cannot have more than %d characters", maxClusterNameLength)),
		)
	}

	if cp.Spec.EnableAutopilot && cp.Spec.ReleaseChannel == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "ReleaseChannel"), "Release channel is required for an autopilot enabled cluster"))
	}

	if cp.Spec.IPAllocationPolicy != nil {
		allErrs = append(allErrs, validateIPAllocationPolicy(cp.Spec)...)
	}
	if cp.Spec.DefaultMaxPodsConstraint != nil {
		allErrs = append(allErrs, validateDefaultMaxPodsConstraint(cp.Spec)...)
	}
	if cp.Spec.MaintenancePolicy != nil {
		allErrs = append(allErrs, validateMaintenancePolicy(cp.Spec)...)
	}
	if cp.Spec.PrivateClusterConfig != nil {
		allErrs = append(allErrs, validatePrivateClusterConfig(cp.Spec)...)
	}
	return allErrs
}

func generateGKEName(resourceName, namespace string, maxLength int) (string, error) {
	escapedName := strings.ReplaceAll(resourceName, ".", "-")
	gkeName := fmt.Sprintf("%s-%s", namespace, escapedName)

	if len(gkeName) < maxLength {
		return gkeName, nil
	}

	hashLength := 32 - len(resourcePrefix)
	hashedName, err := hash.Base36TruncatedHash(gkeName, hashLength)
	if err != nil {
		return "", errors.Wrap(err, "creating hash from name")
	}

	return fmt.Sprintf("%s%s", resourcePrefix, hashedName), nil
}

// validateDefaultMaxPodsConstraint validates that GCPManagedControlPlaneSpec.DefaultMaxPodsConstraint
// is correctly configured.
func validateDefaultMaxPodsConstraint(spec GCPManagedControlPlaneSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.DefaultMaxPodsConstraint == nil {
		return allErrs
	}

	path := field.NewPath("spec", "DefaultMaxPodsConstraint")
	if spec.IPAllocationPolicy == nil || !pointer.BoolDeref(spec.IPAllocationPolicy.UseIPAliases, false) {
		allErrs = append(allErrs,
			field.Invalid(path,
				spec.DefaultMaxPodsConstraint, "field cannot be set unless UseIPAliases is set to true"),
		)
	}

	return allErrs
}

// validateIPAllocationPolicy validates that GCPManagedControlPlaneSpec.IPAllocationPolicy is correctly
// configured and checks fields that are not allowed to be set simultaneously.
func validateIPAllocationPolicy(spec GCPManagedControlPlaneSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.IPAllocationPolicy == nil {
		return allErrs
	}

	path := field.NewPath("spec", "IPAllocationPolicy")

	isUseIPAliases := pointer.BoolDeref(spec.IPAllocationPolicy.UseIPAliases, false)
	if spec.IPAllocationPolicy.ClusterSecondaryRangeName != nil && !isUseIPAliases {
		allErrs = append(allErrs,
			field.Invalid(path.Child("ClusterSecondaryRangeName"),
				spec.IPAllocationPolicy.ClusterSecondaryRangeName,
				"field cannot be set unless UseIPAliases is set to true"),
		)
	}
	if spec.IPAllocationPolicy.ServicesSecondaryRangeName != nil && !isUseIPAliases {
		allErrs = append(allErrs,
			field.Invalid(path.Child("ServicesSecondaryRangeName"),
				spec.IPAllocationPolicy.ServicesSecondaryRangeName,
				"field cannot be set unless UseIPAliases is set to true"),
		)
	}
	if spec.IPAllocationPolicy.ServicesIpv4CidrBlock != nil && !isUseIPAliases {
		allErrs = append(allErrs,
			field.Invalid(path.Child("ServicesIpv4CidrBlock"),
				spec.IPAllocationPolicy.ServicesIpv4CidrBlock,
				"field cannot be set unless UseIPAliases is set to true"),
		)
	}
	if spec.IPAllocationPolicy.ClusterIpv4CidrBlock != nil && !isUseIPAliases {
		allErrs = append(allErrs,
			field.Invalid(path.Child("ClusterIpv4CidrBlock"),
				spec.IPAllocationPolicy.ClusterIpv4CidrBlock,
				"field cannot be set unless UseIPAliases is set to true"),
		)
	}
	if spec.IPAllocationPolicy.ClusterIpv4CidrBlock != nil && spec.ClusterIpv4Cidr != nil {
		allErrs = append(allErrs,
			field.Invalid(path.Child("ClusterIpv4CidrBlock"),
				spec.IPAllocationPolicy.ClusterIpv4CidrBlock,
				"only one of spec.ClusterIpv4Cidr and spec.IPAllocationPolicy.ClusterIpv4CidrBlock can be set"),
		)
	}

	return allErrs
}

// validateMaintenancePolicy validates that GCPManagedControlPlaneSpec.MaintenancePolicy is correctly configured
// and ensures that all TimeWindows used are also validated.
func validateMaintenancePolicy(spec GCPManagedControlPlaneSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.MaintenancePolicy == nil {
		return allErrs
	}

	path := field.NewPath("spec", "MaintenancePolicy")

	if spec.MaintenancePolicy.DailyMaintenanceWindow != nil && spec.MaintenancePolicy.RecurringMaintenanceWindow != nil {
		allErrs = append(allErrs,
			field.Invalid(path.Child("DailyMaintenanceWindow"),
				spec.MaintenancePolicy.DailyMaintenanceWindow,
				"only one of spec.MaintenancePolicy.DailyMaintenanceWindow and spec.MaintenancePolicy.RecurringMaintenanceWindow can be set"),
		)
	}

	if spec.MaintenancePolicy.RecurringMaintenanceWindow != nil {
		allErrs = append(allErrs,
			validateTimeWindow(spec.MaintenancePolicy.RecurringMaintenanceWindow.Window,
				path.Child("RecurringMaintenanceWindow"))...)
	}

	if len(spec.MaintenancePolicy.MaintenanceExclusions) > 0 {
		noUpgradeExclusionsCount := 0
		for _, window := range spec.MaintenancePolicy.MaintenanceExclusions {
			allErrs = append(allErrs, validateTimeWindow(window, path.Child("MaintenanceExclusions"))...)
			// Nil values count as "no-upgrades" by default
			if window.MaintenanceExclusionOption == nil || *window.MaintenanceExclusionOption == NoUpgrades {
				noUpgradeExclusionsCount++
			}
		}
		if noUpgradeExclusionsCount > 3 {
			allErrs = append(allErrs,
				field.Invalid(path.Child("MaintenanceExclusions"),
					spec.MaintenancePolicy.MaintenanceExclusions,
					"maximum of 3 `no-upgrades` maintenance exclusions allowed to be specified"),
			)
		}
	}

	return allErrs
}

// validatePrivateClusterConfig validates that GCPManagedControlPlaneSpec.PrivateClusterConfig is correctly configured.
func validatePrivateClusterConfig(spec GCPManagedControlPlaneSpec) field.ErrorList {
	var allErrs field.ErrorList

	if spec.PrivateClusterConfig == nil {
		return allErrs
	}

	path := field.NewPath("spec", "PrivateClusterConfig")

	if spec.PrivateClusterConfig.EnablePrivateEndpoint && spec.MasterAuthorizedNetworksConfig == nil {
		allErrs = append(allErrs,
			field.Invalid(path.Child("EnablePrivateEndpoint"),
				spec.PrivateClusterConfig.EnablePrivateEndpoint,
				"spec.MasterAuthorizedNetworksConfig must be set if EnablePrivateEndpoint is true"),
		)
	}

	if spec.PrivateClusterConfig.EnablePrivateEndpoint && spec.MasterAuthorizedNetworksConfig != nil &&
		pointer.BoolDeref(spec.MasterAuthorizedNetworksConfig.GcpPublicCidrsAccessEnabled, false) {
		allErrs = append(allErrs,
			field.Invalid(path.Child("EnablePrivateEndpoint"),
				spec.PrivateClusterConfig.EnablePrivateEndpoint,
				"spec.MasterAuthorizedNetworksConfig.GcpPublicCidrsAccessEnabled cannot be true if EnablePrivateEndpoint is true"),
		)
	}

	return allErrs
}

// validateTimeWindow validates a given TimeWindow is correctly configured.
// Errors returned in the error list use the passed in path.
func validateTimeWindow(window *TimeWindow, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if window == nil {
		return allErrs
	}

	startTime, stErr := time.Parse(time.RFC3339, window.StartTime)
	if stErr != nil {
		allErrs = append(allErrs, field.Invalid(path, window.StartTime, "StartTime must be a valid RFC3339 timestamp"))
	}
	endTime, etErr := time.Parse(time.RFC3339, window.EndTime)
	if etErr != nil {
		allErrs = append(allErrs, field.Invalid(path, window.EndTime, "EndTime must be a valid RFC3339 timestamp"))
	}
	if stErr == nil && etErr == nil && startTime.After(endTime) {
		allErrs = append(allErrs, field.Invalid(path, window.StartTime, "StartTime cannot be after EndTime"))
	}

	return allErrs
}
