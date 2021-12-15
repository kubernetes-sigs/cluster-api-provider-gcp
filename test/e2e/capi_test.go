//go:build e2e
// +build e2e

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

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("Running the Cluster API E2E tests", func() {
	var (
		ctx = context.TODO()
	)

	BeforeEach(func() {
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))
	})

	AfterEach(func() {
	})

	Context("Running the quick-start spec", func() {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
			}
		})
	})
})
