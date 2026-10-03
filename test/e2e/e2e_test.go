//go:build e2e
// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx                 = context.TODO()
		specName            = "create-workload-cluster"
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterNamePrefix   string
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))
		Expect(e2eConfig.Variables).To(HaveKey(CCMPath))

		clusterNamePrefix = fmt.Sprintf("capg-e2e-%s", util.RandomString(6))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:             specName,
			Cluster:              result.Cluster,
			ClusterProxy:         bootstrapClusterProxy,
			ClusterctlConfigPath: clusterctlConfigPath,
			Namespace:            namespace,
			CancelWatches:        cancelWatches,
			IntervalsGetter:      e2eConfig.GetIntervals,
			SkipCleanup:          skipCleanup,
			ArtifactFolder:       artifactFolder,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	Context("Creating a single control-plane cluster", func() {
		It("Should create a cluster with 1 worker node and can be scaled", func() {
			clusterName := fmt.Sprintf("%s-single", clusterNamePrefix)
			By("Initializes with 1 worker node")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   clusterctl.DefaultFlavor,
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)

			By("Scaling worker node to 3")
			Expect(result.MachineDeployments).To(HaveLen(1))
			framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				MachineDeployment:         result.MachineDeployments[0],
				Replicas:                  3,
				WaitForMachineDeployments: e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
		})
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 2 worker nodes", func() {
			clusterName := fmt.Sprintf("%s-ha", clusterNamePrefix)
			By("Creating a high available cluster")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   clusterctl.DefaultFlavor,
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](3),
					WorkerMachineCount:       ptr.To[int64](2),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	Context("Creating a single control-plane cluster with per cluster credentials", func() {
		It("Should create a cluster with 1 worker node", func() {
			clusterName := fmt.Sprintf("%s-with-creds", clusterNamePrefix)
			By("Create the credentials secret")

			credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
			Expect(credsFile).NotTo(BeEmpty())
			data, err := os.ReadFile(credsFile)
			Expect(err).NotTo(HaveOccurred())
			secretData := map[string][]byte{
				"credentials": data,
			}
			err = createSecret(ctx, "test-creds", "default", secretData, bootstrapClusterProxy)
			Expect(err).NotTo(HaveOccurred(), "failed creating credentials sercret")

			By("Initializes with 1 worker node")

			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-creds",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	Context("Creating a control-plane cluster with an external and an internal load balancer", func() {
		It("Should create a cluster with 1 control-plane and 1 worker node with an external and an internal load balancer", func() {
			clusterName := fmt.Sprintf("%s-internal-lb", clusterNamePrefix)
			By("Creating a cluster with an internal load balancer and an external load balancer")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-external-and-internal-lb",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	// Skipping the test case for now. The internal load balancer test case requires the management cluster
	// to have access to network that the cluster being created is in.
	// An option to reach that is to use a GKE cluster as the management cluster.
	Context("Creating a control-plane cluster with an internal load balancer", func() {
		It("Should create a cluster with 1 control-plane and 1 worker node with an internal load balancer", func() {
			Skip("This test requires a bootstrap cluster that has access to the network where the cluster is being created.")

			clusterName := fmt.Sprintf("%s-internal-lb", clusterNamePrefix)
			By("Creating a cluster with internal load balancer")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-internal-lb",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	// Skipping the test case for now. The internal load balancer test case requires the management cluster
	// to have access to network that the cluster being created is in.
	// An option to reach that is to use a GKE cluster as the management cluster.
	Context("Creating a control-plane cluster with three control plane nodes and an internal load balancer", func() {
		It("Should create a cluster with 3 control-plane and 1 worker node with an internal load balancer", func() {
			Skip("This test requires a bootstrap cluster that has access to the network where the cluster is being created.")

			clusterName := fmt.Sprintf("%s-internal-lb", clusterNamePrefix)
			By("Creating a cluster with internal load balancer from GKE bootstrap cluster")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-internal-lb",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](3),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	Context("Creating a cluster using a cluster class", func() {
		It("Should create a cluster class and then a cluster based on it", func() {
			clusterName := fmt.Sprintf("%s-topology", clusterNamePrefix)
			By("Creating a cluster from a topology")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-topology",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)
		})
	})

	Context("Creating a cluster with firewall rules provided", func() {
		It("Should create a cluster with the firewall rules provided", func() {
			clusterName := fmt.Sprintf("%s-firewall-rules", clusterNamePrefix)
			By("Creating a cluster with firewall rules provided")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-firewall-rules",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, result)

			gcpClusterKey := client.ObjectKey{Namespace: namespace.Name, Name: clusterName}
			gcpCluster := &infrav1.GCPCluster{}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx, gcpClusterKey, gcpCluster)).To(Succeed())

			By("Naming the rule that was provided without a name")
			rules := gcpCluster.Spec.Network.Firewall.FirewallRules
			Expect(rules).To(HaveLen(2), "the template provides one named and one unnamed rule")
			generatedName := rules[1].Name
			Expect(generatedName).To(HavePrefix(clusterName+"-"), "an omitted name is generated from the cluster name")
			Expect(len(generatedName)).To(BeNumerically("<=", 63), "GCP rejects firewall rule names longer than 63 characters")

			By("Recording every rule it created in the status")
			recorded := gcpCluster.Status.Network.FirewallRules
			// A generated name already carries the cluster name, so it is recorded as is.
			Expect(recorded).To(HaveKey(generatedName))
			// A user provided name has the cluster name prepended and is then
			// truncated to fit, so it is recorded under that name instead.
			providedName := clusterName + "-" + rules[0].Name
			providedName = strings.TrimSuffix(providedName[:min(len(providedName), 63)], "-")
			Expect(recorded).To(HaveKey(providedName))

			By("Keeping the generated name stable across reconciles")
			// A name that changed between reconciles would orphan the rule the
			// previous reconcile created, so the rule has to keep the name it
			// was first given rather than being regenerated.
			Consistently(func() (string, error) {
				cluster := &infrav1.GCPCluster{}
				if err := bootstrapClusterProxy.GetClient().Get(ctx, gcpClusterKey, cluster); err != nil {
					return "", err
				}
				return cluster.Spec.Network.Firewall.FirewallRules[1].Name, nil
			}, "1m", "5s").Should(Equal(generatedName))

			By("Deleting a rule that is removed from the spec")
			patchHelper, err := patch.NewHelper(gcpCluster, bootstrapClusterProxy.GetClient())
			Expect(err).NotTo(HaveOccurred())
			gcpCluster.Spec.Network.Firewall.FirewallRules = rules[:1]
			Expect(patchHelper.Patch(ctx, gcpCluster)).To(Succeed())

			Eventually(func() (map[string]string, error) {
				cluster := &infrav1.GCPCluster{}
				if err := bootstrapClusterProxy.GetClient().Get(ctx, gcpClusterKey, cluster); err != nil {
					return nil, err
				}
				return cluster.Status.Network.FirewallRules, nil
			}, e2eConfig.GetIntervals(specName, "wait-cluster")...).ShouldNot(HaveKey(generatedName))
		})
	})
})
