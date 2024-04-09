/*
Copyright 2023 The Kubernetes Authors.

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

package cloud

const (
	// CustomDataHashAnnotation is the key for the machine object annotation
	// which tracks the hash of the custom data.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// for annotation formatting rules.
	CustomDataHashAnnotation = "sigs.k8s.io/cluster-api-provider-gcp-mig-custom-data-hash"

	// ClusterAPIImagePrefix is the prefix for the image name used by the Cluster API provider for GCP.
	ClusterAPIImagePrefix = "capi-ubuntu-1804-k8s-"
)
