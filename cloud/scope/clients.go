/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"
	"time"

	computerest "cloud.google.com/go/compute/apiv1"
	container "cloud.google.com/go/container/apiv1"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/util/flowcontrol"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GCPServices contains all the gcp services used by the scopes.
type GCPServices struct {
	Compute *compute.Service
}

// GCPRateLimiter implements cloud.RateLimiter.
type GCPRateLimiter struct{}

// credentialHeader is a helper struct used for determining the type of
// GCP credentials from JSON data.
type credentialHeader struct {
	Type string `json:"type"`
}

// Accept blocks until the operation can be performed.
func (rl *GCPRateLimiter) Accept(ctx context.Context, key *cloud.RateLimitKey) error {
	if key.Operation == "Get" && key.Service == "Operations" {
		// Wait a minimum amount of time regardless of rate limiter.
		rl := &cloud.MinimumRateLimiter{
			// Convert flowcontrol.RateLimiter into cloud.RateLimiter
			RateLimiter: &cloud.AcceptRateLimiter{
				Acceptor: flowcontrol.NewTokenBucketRateLimiter(5, 5), // 5
			},
			Minimum: time.Second,
		}

		return rl.Accept(ctx, key)
	}
	return nil
}

// Observe does nothing.
func (rl *GCPRateLimiter) Observe(context.Context, error, *cloud.RateLimitKey) {
	// noop
}

func newCloud(project string, service GCPServices) cloud.Cloud {
	return cloud.NewGCE(&cloud.Service{
		GA:            service.Compute,
		ProjectRouter: &cloud.SingleProjectRouter{ID: project},
		RateLimiter:   &GCPRateLimiter{},
	})
}

// defaultClientOptions returns the client options shared by every GCP client,
// along with the universe domain those options resolve to. Callers that build
// an endpoint themselves need the universe domain so they do not have to
// re-resolve the credentials to get it.
//
// Credentials are resolved even when credentialsRef is nil, because the
// clients would otherwise fall back to Application Default Credentials on
// their own and the universe domain has to come from whichever credentials
// they end up using. option.WithUniverseDomain takes precedence over the
// GOOGLE_CLOUD_UNIVERSE_DOMAIN environment variable, so pinning it here means
// a sovereign (GCD) universe is detected from the credentials alone rather
// than from the controller's environment.
func defaultClientOptions(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client) ([]option.ClientOption, string, error) {
	opts := []option.ClientOption{
		option.WithUserAgent(fmt.Sprintf("gcp.cluster.x-k8s.io/%s", version.Get())),
	}

	resolved, err := CredentialsClient{}.Resolve(ctx, credentialsRef, crClient)
	if err != nil {
		if credentialsRef == nil {
			return nil, "", fmt.Errorf("getting application default gcp credentials: %w", err)
		}
		return nil, "", fmt.Errorf("getting gcp credentials from reference %s: %w", credentialsRef, err)
	}

	// Only an explicit reference injects its credential JSON into the clients.
	// Implicit ADC is still left for them to resolve themselves.
	if credentialsRef != nil {
		var optCredType option.CredentialsType
		switch resolved.Type {
		case google.ExternalAccount:
			optCredType = option.ExternalAccount
		case google.ImpersonatedServiceAccount:
			optCredType = option.ImpersonatedServiceAccount
		default:
			optCredType = option.ServiceAccount
		}
		opts = append(opts, option.WithAuthCredentialsJSON(optCredType, resolved.RawData))
	}

	// Extract universe domain for sovereign clouds.
	universeDomain, err := resolved.Credentials.GetUniverseDomain()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get universe domain from credentials: %w", err)
	}
	opts = append(opts, option.WithUniverseDomain(universeDomain))

	return opts, universeDomain, nil
}

// tagBindingsEndpoint returns the Cloud Resource Manager endpoint to dial for
// location in universeDomain. An explicit ResourceManagerServiceEndpoint
// override always wins; otherwise the regional endpoint is derived from the
// universe domain. Tag bindings have no universe-aware default endpoint in the
// client library because the endpoint is regional, so CAPG builds it here
// instead of letting the library derive it.
func tagBindingsEndpoint(location, universeDomain string, endpoints *infrav1.ServiceEndpoints) string {
	if endpoints != nil && endpoints.ResourceManagerServiceEndpoint != "" {
		return endpoints.ResourceManagerServiceEndpoint
	}

	if universeDomain == "" {
		universeDomain = DefaultUniverseDomain
	}

	return fmt.Sprintf("%s-cloudresourcemanager.%s:443", location, universeDomain)
}

func newComputeService(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client, endpoints *infrav1.ServiceEndpoints) (*compute.Service, error) {
	opts, universeDomain, err := defaultClientOptions(ctx, credentialsRef, crClient)
	if err != nil {
		return nil, fmt.Errorf("getting default gcp client options: %w", err)
	}
	// The compute service is the one client every non-GKE scope builds, so this
	// reports the universe domain once per scope rather than once per client.
	logUniverseDomain(ctx, universeDomain, endpoints)

	if endpoints != nil && endpoints.ComputeServiceEndpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoints.ComputeServiceEndpoint))
	}

	computeSvc, err := compute.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating new compute service instance: %w", err)
	}

	return computeSvc, nil
}

func newClusterManagerClient(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client, endpoints *infrav1.ServiceEndpoints) (*container.ClusterManagerClient, error) {
	opts, universeDomain, err := defaultClientOptions(ctx, credentialsRef, crClient)
	if err != nil {
		return nil, fmt.Errorf("getting default gcp client options: %w", err)
	}
	// Likewise for GKE scopes, which build a cluster manager client rather than
	// a compute service.
	logUniverseDomain(ctx, universeDomain, endpoints)

	if endpoints != nil && endpoints.ContainerServiceEndpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoints.ContainerServiceEndpoint))
	}

	managedClusterClient, err := container.NewClusterManagerClient(ctx, opts...)
	if err != nil {
		return nil, errors.Errorf("failed to create gcp cluster manager client: %v", err)
	}

	return managedClusterClient, nil
}

func newInstanceGroupManagerClient(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client, endpoints *infrav1.ServiceEndpoints) (*computerest.InstanceGroupManagersClient, error) {
	opts, _, err := defaultClientOptions(ctx, credentialsRef, crClient)
	if err != nil {
		return nil, fmt.Errorf("getting default gcp client options: %w", err)
	}

	if endpoints != nil && endpoints.ComputeServiceEndpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoints.ComputeServiceEndpoint))
	}

	instanceGroupManagersClient, err := computerest.NewInstanceGroupManagersRESTClient(ctx, opts...)
	if err != nil {
		return nil, errors.Errorf("failed to create gcp instance group managers rest client: %v", err)
	}

	return instanceGroupManagersClient, nil
}

func newInstanceTemplatesRESTClient(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client, endpoints *infrav1.ServiceEndpoints) (*computerest.InstanceTemplatesClient, error) {
	opts, _, err := defaultClientOptions(ctx, credentialsRef, crClient)
	if err != nil {
		return nil, fmt.Errorf("getting default gcp client options: %w", err)
	}

	if endpoints != nil && endpoints.ComputeServiceEndpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoints.ComputeServiceEndpoint))
	}

	instanceTemplatesClient, err := computerest.NewInstanceTemplatesRESTClient(ctx, opts...)
	if err != nil {
		return nil, errors.Errorf("failed to create gcp instance templates rest client: %v", err)
	}

	return instanceTemplatesClient, nil
}

func newTagBindingsClient(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client, location string, endpoints *infrav1.ServiceEndpoints) (*resourcemanager.TagBindingsClient, error) {
	opts, universeDomain, err := defaultClientOptions(ctx, credentialsRef, crClient)
	if err != nil {
		return nil, fmt.Errorf("getting default gcp client options: %w", err)
	}

	opts = append(opts, option.WithEndpoint(tagBindingsEndpoint(location, universeDomain, endpoints)))

	client, err := resourcemanager.NewTagBindingsClient(ctx, opts...)
	if err != nil {
		return nil, errors.Errorf("failed to create gcp tag binding client: %v", err)
	}

	return client, nil
}
