/*
Copyright 2026 The Kubernetes Authors.

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

package scope

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultUniverseDomain is the default universe domain for GCP.
	DefaultUniverseDomain = "googleapis.com"
)

// IsNonDefaultUniverseDomain checks if the given universe domain differs from the default.
func IsNonDefaultUniverseDomain(universeDomain string) bool {
	return universeDomain != "" && universeDomain != DefaultUniverseDomain
}

// logUniverseDomain reports the universe domain a client was built for, so
// operators of a sovereign (GCD) environment can confirm it was picked up from
// the credentials. Nothing is logged for the default universe domain, which is
// the overwhelmingly common case and would only be noise.
//
// It also warns when no ServiceEndpoints are configured, since the universe
// domain alone does not resolve an endpoint for every service and such an
// environment may need explicit overrides.
func logUniverseDomain(ctx context.Context, universeDomain string, endpoints *infrav1.ServiceEndpoints) {
	if !IsNonDefaultUniverseDomain(universeDomain) {
		return
	}

	log := log.FromContext(ctx).WithValues("universeDomain", universeDomain)
	log.Info("Operating in a non-default universe domain")

	if endpoints == nil {
		log.Info("No ServiceEndpoints are configured for a non-default universe domain. API calls may fail if this environment requires explicit endpoint overrides")
	}
}
