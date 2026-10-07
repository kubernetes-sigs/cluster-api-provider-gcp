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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
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

			DeferCleanup(func() {
				// Dump autoscaler logs on test failure
				if CurrentSpecReport().Failed() {
					pods, err := bootstrapClusterProxy.GetClientSet().CoreV1().Pods(namespace.Name).
						List(ctx, metav1.ListOptions{LabelSelector: "app=cluster-autoscaler"})
					if err != nil || len(pods.Items) == 0 {
						fmt.Fprintf(GinkgoWriter, "Failed to find autoscaler pod for logs: %v\n", err)
						return
					}
					autoscalerLogs, err := bootstrapClusterProxy.GetClientSet().CoreV1().Pods(namespace.Name).
						GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{TailLines: ptr.To[int64](200)}).
						Do(ctx).Raw()
					if err != nil {
						fmt.Fprintf(GinkgoWriter, "Failed to get autoscaler logs on cleanup: %v\n", err)
					} else {
						fmt.Fprintf(GinkgoWriter, "\n===== AUTOSCALER LOGS (last 200 lines) =====\n%s\n", string(autoscalerLogs))
					}
				}
			})

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

			By("Verifying GCPMachineTemplate status is populated for scale-from-zero")
			Expect(result.MachineDeployments).To(HaveLen(1))
			md := result.MachineDeployments[0]
			templateRef := md.Spec.Template.Spec.InfrastructureRef
			template := &infrav1.GCPMachineTemplate{}

			Eventually(func(g Gomega) {
				err := bootstrapClusterProxy.GetClient().Get(ctx,
					client.ObjectKey{Namespace: md.Namespace, Name: templateRef.Name},
					template)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(template.Status.Capacity).NotTo(BeNil(), "Status.Capacity should be populated")
				g.Expect(template.Status.Capacity.Cpu().IsZero()).To(BeFalse(), "CPU capacity should be set")
				g.Expect(template.Status.Capacity.Memory().IsZero()).To(BeFalse(), "Memory capacity should be set")

				g.Expect(template.Status.NodeInfo).NotTo(BeNil(), "Status.NodeInfo should be populated")
				g.Expect(template.Status.NodeInfo.Architecture).To(BeElementOf(infrav1.ArchitectureAmd64, infrav1.ArchitectureArm64), "Architecture should be amd64 or arm64")
				g.Expect(template.Status.NodeInfo.OperatingSystem).To(Equal(corev1.Linux), "OperatingSystem should be linux")
			}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Succeed())

			By("Scaling worker node to 3")
			Expect(result.MachineDeployments).To(HaveLen(1))
			framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				MachineDeployment:         result.MachineDeployments[0],
				Replicas:                  3,
				WaitForMachineDeployments: e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})

			// Scale-from-zero validation using same MachineDeployment
			By("Scaling MachineDeployment to 0 for scale-from-zero test")
			mgmtClient := bootstrapClusterProxy.GetClient()
			workloadCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

			// Add autoscaler annotations to existing MachineDeployment
			md = result.MachineDeployments[0]
			Eventually(func(g Gomega) {
				currentMD := &clusterv1.MachineDeployment{}
				g.Expect(mgmtClient.Get(ctx, client.ObjectKey{
					Namespace: md.Namespace,
					Name:      md.Name,
				}, currentMD)).To(Succeed())

				// Add autoscaler annotations
				if currentMD.Annotations == nil {
					currentMD.Annotations = make(map[string]string)
				}
				currentMD.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"] = "0"
				currentMD.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"] = "3"

				g.Expect(mgmtClient.Update(ctx, currentMD)).To(Succeed())
			}, "30s", "1s").Should(Succeed())

			// Scale to 0
			framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   result.Cluster,
				MachineDeployment:         md,
				Replicas:                  0,
				WaitForMachineDeployments: e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})

			By("Verifying GCPMachineTemplate Status remains populated at 0 replicas")
			Eventually(func(g Gomega) {
				templateRef := md.Spec.Template.Spec.InfrastructureRef
				template := &infrav1.GCPMachineTemplate{}
				err := mgmtClient.Get(ctx,
					client.ObjectKey{Namespace: md.Namespace, Name: templateRef.Name},
					template)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(template.Status.Capacity).NotTo(BeNil(), "Status.Capacity should be populated at 0 replicas")
				g.Expect(template.Status.Capacity.Cpu().IsZero()).To(BeFalse(), "CPU capacity should be set")
				g.Expect(template.Status.Capacity.Memory().IsZero()).To(BeFalse(), "Memory capacity should be set")

				g.Expect(template.Status.NodeInfo).NotTo(BeNil(), "Status.NodeInfo should be populated")
				g.Expect(template.Status.NodeInfo.Architecture).To(BeElementOf(infrav1.ArchitectureAmd64, infrav1.ArchitectureArm64))
				g.Expect(template.Status.NodeInfo.OperatingSystem).To(Equal(corev1.Linux))
			}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Succeed())

			By("Deploying cluster-autoscaler RBAC to management cluster")
			Expect(deployClusterAutoscalerRBAC(ctx, mgmtClient, namespace.Name, clusterName)).To(Succeed())

			By("Deploying cluster-autoscaler to management cluster")
			Expect(deployClusterAutoscaler(ctx, mgmtClient, namespace.Name, clusterName, e2eConfig.MustGetVariable(ClusterAutoscalerVersion))).To(Succeed())

			By("Waiting for cluster-autoscaler pod to be ready")
			Eventually(func(g Gomega) {
				pods, err := bootstrapClusterProxy.GetClientSet().CoreV1().Pods(namespace.Name).
					List(ctx, metav1.ListOptions{LabelSelector: "app=cluster-autoscaler"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pods.Items).To(HaveLen(1))
				g.Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))

				hasReadyCondition := false
				for _, cond := range pods.Items[0].Status.Conditions {
					if cond.Type == corev1.PodReady {
						hasReadyCondition = true
						g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))
						break
					}
				}
				g.Expect(hasReadyCondition).To(BeTrue(), "Pod Ready condition not found")
			}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

			By("Creating trigger workload to force autoscaler scale-up")
			triggerDeployment, err := createTriggerWorkload(ctx, workloadCluster.GetClient(), clusterName)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying trigger pods are pending (no nodes available)")
			Eventually(func(g Gomega) {
				pods := &corev1.PodList{}
				err := workloadCluster.GetClient().List(ctx, pods,
					client.InNamespace("default"),
					client.MatchingLabels{"app": "autoscale-trigger"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pods.Items).To(HaveLen(1))
				g.Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodPending), "Pod should be pending with 0 nodes")
			}, "10s", "1s").Should(Succeed())

			By("Validating autoscaler scales MachineDeployment from 0 to 1")
			Eventually(func(g Gomega) {
				currentMD := &clusterv1.MachineDeployment{}
				err := mgmtClient.Get(ctx, client.ObjectKey{Namespace: md.Namespace, Name: md.Name}, currentMD)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*currentMD.Spec.Replicas).To(BeNumerically(">", 0), "Autoscaler should scale MD replicas > 0")
			}, "5m", "5s").Should(Succeed())

			By("Waiting for node to become ready")
			Eventually(func(g Gomega) {
				currentMD := &clusterv1.MachineDeployment{}
				err := mgmtClient.Get(ctx, client.ObjectKey{Namespace: md.Namespace, Name: md.Name}, currentMD)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(currentMD.Status.ReadyReplicas).To(HaveValue(BeNumerically(">=", 1)), "MD should have ready replica")
			}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Succeed())

			By("Verifying trigger pod becomes Running")
			Eventually(func(g Gomega) {
				pods := &corev1.PodList{}
				err := workloadCluster.GetClient().List(ctx, pods,
					client.InNamespace("default"),
					client.MatchingLabels{"app": "autoscale-trigger"})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pods.Items).To(HaveLen(1))
				g.Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
			}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

			By("Cleaning up autoscaler resources")
			Expect(workloadCluster.GetClient().Delete(ctx, triggerDeployment)).To(Succeed())

			autoscalerDep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("cluster-autoscaler-%s", clusterName),
					Namespace: namespace.Name,
				},
			}
			Expect(mgmtClient.Delete(ctx, autoscalerDep)).To(Succeed())

			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("cluster-autoscaler-management-%s", clusterName),
				},
			}
			Eventually(func(g Gomega) {
				err := mgmtClient.Delete(ctx, clusterRole)
				g.Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue())
			}, "10s", "1s").Should(Succeed())

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("cluster-autoscaler-management-%s", clusterName),
				},
			}
			Eventually(func(g Gomega) {
				err := mgmtClient.Delete(ctx, clusterRoleBinding)
				g.Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue())
			}, "10s", "1s").Should(Succeed())
		})
	})

	Context("Creating a single control-plane cluster with MachinePool and ip alias", func() {
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
					Flavor:                   "ci-with-machinepool-ip-alias",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			}, result)

			By("Scaling worker node to 3")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-machinepool-ip-alias",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To[int64](1),
					WorkerMachineCount:       ptr.To[int64](3),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
				WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
			}, result)
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
		})
	})
})
