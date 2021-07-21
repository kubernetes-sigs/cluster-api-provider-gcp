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
	"google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1beta1"
)

// GCPClients contains all the gcp clients used by the scopes.
type GCPClients struct {
	Compute *compute.Service
}

type GKEClients struct {
	Container *container.Service
}