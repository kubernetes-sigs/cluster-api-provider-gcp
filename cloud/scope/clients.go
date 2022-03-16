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
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"google.golang.org/api/compute/v1"
	"k8s.io/client-go/util/flowcontrol"
)

// GCPServices contains all the gcp services used by the scopes.
type GCPServices struct {
	Compute *compute.Service
}

// GCPRateLimiter implements cloud.RateLimiter.
type GCPRateLimiter struct{}

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

func newCloud(project string, service GCPServices) cloud.Cloud {
	return cloud.NewGCE(&cloud.Service{
		GA:            service.Compute,
		ProjectRouter: &cloud.SingleProjectRouter{ID: project},
		RateLimiter:   &GCPRateLimiter{},
	})
}
