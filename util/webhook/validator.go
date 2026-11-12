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

// Package webhook implements reusable validation functions for webhooks.
package webhook

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

const (
	unsetMessage     = "field is immutable, unable to set an empty value if it was already set"
	setMessage       = "field is immutable, unable to assign a value if it was already empty"
	immutableMessage = "field is immutable"
)

// ValidateImmutable validates equality across two values,
// and returns a meaningful error to indicate a changed value, a newly set value, or a newly unset value.
func ValidateImmutable(path *field.Path, oldVal, newVal any) *field.Error {
	if reflect.TypeOf(oldVal) != reflect.TypeOf(newVal) {
		return field.Invalid(path, newVal, "unexpected error")
	}
	if !reflect.ValueOf(oldVal).IsZero() {
		// Prevent modification if it was already set to some value
		if reflect.ValueOf(newVal).IsZero() {
			// unsetting the field is not allowed
			return field.Invalid(path, newVal, unsetMessage)
		}
		if !reflect.DeepEqual(oldVal, newVal) {
			// changing the field is not allowed
			return field.Invalid(path, newVal, immutableMessage)
		}
	} else if !reflect.ValueOf(newVal).IsZero() {
		return field.Invalid(path, newVal, setMessage)
	}

	return nil
}

// ValidateNonNegative checks if a numeric value is non-negative,
// and returns a meaningul error to indicate that it is.
func ValidateNonNegative[T int | int32 | int64](path *field.Path, value *T) *field.Error {
	if value != nil && *value < 0 {
		return field.Invalid(path, value, "must be non-negative")
	}

	return nil
}

// ValidateResourcePolicies ensures that policies are valid, in the correct region and have correct project names.
// Also ensures they are named in line with RFC1035
func ValidateResourcePolicies(policies []string) error {
	for i, policy := range policies {
		resource, err := cloud.ParseResourceURL(policy)
		if err != nil {
			return fmt.Errorf("resourcePolicies[%d] %q: invalid Compute Engine resource path", i, policy)
		}
		if resource.Key == nil || resource.Resource != "resourcePolicies" {
			return fmt.Errorf("resourcePolicies[%d] %q: must reference a resource policy", i, policy)
		}
		if resource.Key.Type() != meta.Regional || !resource.Key.Valid() {
			return fmt.Errorf("resourcePolicies[%d] %q: must reference a policy in a valid region", i, policy)
		}
		if resource.ProjectID == "" && !strings.HasPrefix(policy, "regions/") {
			return fmt.Errorf("resourcePolicies[%d] %q: project must not be empty", i, policy)
		}
		// Compute policy names follow RFC1035, including its 63-character limit.
		if errors := validation.IsDNS1035Label(resource.Key.Name); len(errors) > 0 {
			return fmt.Errorf("resourcePolicies[%d] %q: %s", i, policy, strings.Join(errors, "; "))
		}
	}
	return nil
}

// ValidateReservationAffinity ensures that reservation affinities cannot be set to values the underlying objects
// do not support.
func ValidateReservationAffinity(affinity *infrav1.ReservationAffinity) error {
	if affinity == nil {
		return nil
	}
	if affinity.ConsumeReservationType == infrav1.ReservationConsumeTypeAnyReservationThenFail {
		return fmt.Errorf("%s is a GKE only feature, instances and machine pools only support ANY_RESERVATION, NO_RESERVATION and SPECIFIC_RESERVATION only", infrav1.ReservationConsumeTypeAnyReservationThenFail)
	}
	return nil
}
