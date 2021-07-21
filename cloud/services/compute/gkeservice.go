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

package compute

import (
	container "google.golang.org/api/container/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
)

// Service holds a collection of interfaces.
// The interfaces are broken down like this to group functions together.
// One alternative is to have a large list of functions from the gcp client.
type GKEService struct {
	scope *scope.GKEClusterScope

	// Helper clients for GCP.
	projects       *container.ProjectsService
}

// NewService returns a new service given the gcp api client.
func NewGKEService(scope *scope.GKEClusterScope) *GKEService {
	return &GKEService{
		scope:           scope,
		projects:        scope.Container.Projects,
	}
}
