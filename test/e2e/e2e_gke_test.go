//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

const (
	defaultNumZonesPerRegion = 3
)

var _ = Describe("GKE workload cluster creation", func() {
	var (
		ctx                 = context.TODO()
		specName            = "create-gke-workload-cluster"
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *ApplyManagedClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		clusterName = fmt.Sprintf("capg-e2e-%s", util.RandomString(6))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)

		result = new(ApplyManagedClusterTemplateAndWaitResult)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:        specName,
			Cluster:         result.Cluster,
			ClusterProxy:    bootstrapClusterProxy,
			Namespace:       namespace,
			CancelWatches:   cancelWatches,
			IntervalsGetter: e2eConfig.GetIntervals,
			SkipCleanup:     skipCleanup,
			ArtifactFolder:  artifactFolder,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	Context("Creating a GKE cluster without autopilot", func() {
		It("Should create a cluster with 1 machine pool and scale", func() {
			By("Initializes with 1 machine pool")

			minPoolSize, ok := e2eConfig.Variables["GKE_MACHINE_POOL_MIN"]
			Expect(ok).To(BeTrue(), "must have min pool size set via the GKE_MACHINE_POOL_MIN variable")
			maxPoolSize, ok := e2eConfig.Variables["GKE_MACHINE_POOL_MAX"]
			Expect(ok).To(BeTrue(), "must have max pool size set via the GKE_MACHINE_POOL_MAX variable")

			ApplyManagedClusterTemplateAndWait(ctx, ApplyManagedClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-gke",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](3),
					ClusterctlVariables: map[string]string{
						"GKE_MACHINE_POOL_MIN": minPoolSize,
						"GKE_MACHINE_POOL_MAX": maxPoolSize,
					},
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-worker-machine-pools"),
			}, result)

			By("Scaling the machine pool up")
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				Replicas:                  6,
				MachinePools:              result.MachinePools,
				WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-worker-machine-pools"),
			})

			By("Scaling the machine pool down")
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				Replicas:                  3,
				MachinePools:              result.MachinePools,
				WaitForMachinePoolToScale: e2eConfig.GetIntervals(specName, "wait-worker-machine-pools"),
			})
		})
	})

	Context("Creating a GKE cluster with autopilot", func() {
		It("Should create a cluster with 1 machine pool and scale", func() {
			By("Initializes with 1 machine pool")

			ApplyManagedClusterTemplateAndWait(ctx, ApplyManagedClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-gke-autopilot",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](0),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-worker-machine-pools"),
			}, result)
		})
	})

	Context("Creating a GKE cluster with custom subnet", func() {
		It("Should create a cluster with 3 machine pool and custom subnet", func() {
			By("Initializes with 3 machine pool")

			ApplyManagedClusterTemplateAndWait(ctx, ApplyManagedClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-gke-custom-subnet",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](3),
					ClusterctlVariables: map[string]string{
						"GCP_SUBNET_NAME": "capg-test-subnet",
						"GCP_SUBNET_CIDR": "172.20.0.0/16",
					},
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-worker-machine-pools"),
			}, result)
		})
	})
})
