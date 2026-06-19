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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/feature"
	firewallutil "sigs.k8s.io/cluster-api-provider-gcp/util/firewall"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// clusterlog is for logging in this package.
var clusterlog = logf.Log.WithName("gcpcluster-resource")

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (c *GCPCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.GCPCluster{}).
		WithValidator(c).
		WithDefaulter(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-gcpcluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,versions=v1beta1,name=validation.gcpcluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-gcpcluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,versions=v1beta1,name=default.gcpcluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// GCPCluster implements a validating and defaulting webhook for GCPCluster.
type GCPCluster struct{}

var validLoadBalancerTypes = map[infrav1.LoadBalancerType]struct{}{
	infrav1.External:                 {},
	infrav1.RegionalExternal:         {},
	infrav1.Internal:                 {},
	infrav1.InternalExternal:         {},
	infrav1.RegionalInternalExternal: {},
}

// validateLoadBalancerSpec returns admission warnings and errors for a
// LoadBalancerSpec. The kubebuilder Enum marker on LoadBalancerType only
// enforces the CRD schema; this runs the same check for clients that bypass
// it, plus a cross-field check that catches configuration that is silently
// ignored by the reconciler.
func validateLoadBalancerSpec(lb *infrav1.LoadBalancerSpec, fldPath *field.Path) (admission.Warnings, field.ErrorList) {
	var warnings admission.Warnings
	var allErrs field.ErrorList

	if lb.LoadBalancerType != nil {
		if _, ok := validLoadBalancerTypes[*lb.LoadBalancerType]; !ok {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("loadBalancerType"), *lb.LoadBalancerType, []string{
				string(infrav1.External), string(infrav1.RegionalExternal), string(infrav1.Internal),
				string(infrav1.InternalExternal), string(infrav1.RegionalInternalExternal),
			}))
		}
	}

	if lb.ExternalLoadBalancerConfig != nil && lb.LoadBalancerType != nil && *lb.LoadBalancerType == infrav1.Internal {
		warnings = append(warnings,
			fldPath.Child("externalLoadBalancerConfig").String()+
				" is set but LoadBalancerType is Internal, which has no external component; the field will be ignored")
	}

	return warnings, allErrs
}

// validateRegionalLoadBalancerFeatureGate rejects the LoadBalancerSpec fields
// guarded by the RegionalLoadBalancer feature gate when the gate is disabled.
// It only runs on create: LoadBalancer is immutable, and rejecting updates of
// a cluster created while the gate was enabled would block metadata changes
// such as finalizer removal on deletion.
func validateRegionalLoadBalancerFeatureGate(lb *infrav1.LoadBalancerSpec, fldPath *field.Path) field.ErrorList {
	if feature.Gates.Enabled(feature.RegionalLoadBalancer) {
		return nil
	}

	var allErrs field.ErrorList

	if lb.LoadBalancerType != nil {
		if lbType := *lb.LoadBalancerType; lbType == infrav1.RegionalExternal || lbType == infrav1.RegionalInternalExternal {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("loadBalancerType"),
				fmt.Sprintf("%s can be set only if the RegionalLoadBalancer feature flag is enabled", lbType)))
		}
	}
	if lb.ExternalLoadBalancerConfig != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("externalLoadBalancerConfig"),
			"can be set only if the RegionalLoadBalancer feature flag is enabled"))
	}

	return allErrs
}

var (
	_ admission.Validator[*infrav1.GCPCluster] = &GCPCluster{}
	_ admission.Defaulter[*infrav1.GCPCluster] = &GCPCluster{}
)

func (*GCPCluster) Default(_ context.Context, c *infrav1.GCPCluster) error {
	clusterlog.Info("default", "name", c.Name)

	if firewallutil.SkipRuleNameDefaulting(c) {
		return nil
	}

	if err := firewallutil.DefaultRuleNames(c.Spec.Network.Firewall.FirewallRules, firewallutil.RuleNamePrefix(c)); err != nil {
		clusterlog.Error(err, "failed to generate firewall rule names")
		return err
	}

	return nil
}

func (*GCPCluster) ValidateCreate(_ context.Context, c *infrav1.GCPCluster) (admission.Warnings, error) {
	clusterlog.Info("validate create", "name", c.Name)

	allErrs := firewallutil.ValidateRules(c.Spec.Network.Firewall.FirewallRules,
		field.NewPath("spec", "Network", "Firewall", "FirewallRules"))

	fldPath := field.NewPath("spec", "loadBalancer")
	warnings, lbErrs := validateLoadBalancerSpec(&c.Spec.LoadBalancer, fldPath)
	allErrs = append(allErrs, lbErrs...)
	allErrs = append(allErrs, validateRegionalLoadBalancerFeatureGate(&c.Spec.LoadBalancer, fldPath)...)
	if len(allErrs) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("GCPCluster").GroupKind(), c.Name, allErrs)
}

func (*GCPCluster) ValidateUpdate(_ context.Context, old, c *infrav1.GCPCluster) (admission.Warnings, error) {
	clusterlog.Info("validate update", "name", c.Name)

	// Re-run the semantic LoadBalancer validation on updates. The
	// LoadBalancer field is immutable and blocked below, but running
	// the validation here still catches invalid state that slipped in via a
	// client that bypassed the create webhook (e.g. Go SDK/operator).
	warnings, allErrs := validateLoadBalancerSpec(&c.Spec.LoadBalancer, field.NewPath("spec", "loadBalancer"))

	if !reflect.DeepEqual(c.Spec.Project, old.Spec.Project) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Project"),
				c.Spec.Project, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(c.Spec.Region, old.Spec.Region) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Region"),
				c.Spec.Region, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(c.Spec.CredentialsRef, old.Spec.CredentialsRef) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "CredentialsRef"),
				c.Spec.CredentialsRef, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(c.Spec.LoadBalancer, old.Spec.LoadBalancer) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "LoadBalancer"),
				c.Spec.LoadBalancer, "field is immutable"),
		)
	}

	if c.Spec.Network.Mtu < int64(1300) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Network", "Mtu"),
				c.Spec.Network.Mtu, "field cannot be lesser than 1300"),
		)
	}

	if c.Spec.Network.Mtu > int64(8896) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Network", "Mtu"),
				c.Spec.Network.Mtu, "field cannot be greater than 8896"),
		)
	}

	for i, firewallRule := range c.Spec.Network.Firewall.FirewallRules {
		for j, allowRule := range firewallRule.Allowed {
			if allowRule.IPProtocol != infrav1.FirewallProtocolTCP && allowRule.IPProtocol != infrav1.FirewallProtocolUDP &&
				len(allowRule.Ports) > 0 {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec", "Network", "Firewall",
						fmt.Sprintf("FirewallRules[%d]", i), fmt.Sprintf("Allowed[%d]", j),
					),
						allowRule.Ports,
						"field should not exist unless IPProtocol is TCP or UDP"),
				)
			}
		}
		for j, denyRule := range firewallRule.Denied {
			if denyRule.IPProtocol != infrav1.FirewallProtocolTCP && denyRule.IPProtocol != infrav1.FirewallProtocolUDP &&
				len(denyRule.Ports) > 0 {
				allErrs = append(allErrs,
					field.Invalid(field.NewPath("spec", "Network", "Firewall",
						fmt.Sprintf("FirewallRules[%d]", i), fmt.Sprintf("Denied[%d]", j),
					),
						denyRule.Ports,
						"field should not exist unless IPProtocol is TCP or UDP"),
				)
			}
		}
	}

	// Rules that were admitted before this validation existed are grandfathered in: a
	// cluster whose stored rules cannot be told apart stays updatable as long as the
	// rules are left alone, so an unrelated change is not rejected over a field it does
	// not touch. Modifying the rules at all opts the whole list back into validation.
	if !reflect.DeepEqual(c.Spec.Network.Firewall.FirewallRules, old.Spec.Network.Firewall.FirewallRules) {
		rulesPath := field.NewPath("spec", "Network", "Firewall", "FirewallRules")
		allErrs = append(allErrs, firewallutil.ValidateRules(c.Spec.Network.Firewall.FirewallRules, rulesPath)...)
		allErrs = append(allErrs, firewallutil.ValidateRuleUpdates(old.Spec.Network.Firewall.FirewallRules,
			c.Spec.Network.Firewall.FirewallRules, rulesPath)...)
	}

	if len(allErrs) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("GCPCluster").GroupKind(), c.Name, allErrs)
}

func (*GCPCluster) ValidateDelete(_ context.Context, _ *infrav1.GCPCluster) (admission.Warnings, error) {
	return nil, nil
}
