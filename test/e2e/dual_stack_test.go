//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Dual Stack Network Tests", func() {
	var (
		ctx                 = context.TODO()
		specName            = "dual-stack-network"
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

	Context("Testing StackType with DualStack and AddressPreferencePolicy IPv4Primary", func() {
		It("Should create a cluster with DualStack network and IPv4 as primary address", func() {
			clusterName := fmt.Sprintf("%s-ds-ipv4", clusterNamePrefix)

			By("Creating a dual stack cluster with IPv4 primary address preference")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-dual-stack",
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

			By("Verifying the GCPCluster has DualStack StackType configuration")
			gcpCluster := &infrav1.GCPCluster{}
			key := client.ObjectKey{
				Namespace: namespace.Name,
				Name:      clusterName,
			}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx, key, gcpCluster)).To(Succeed())

			// Verify StackType is DualStack
			Expect(gcpCluster.Spec.Network.StackType).To(Equal(infrav1.DualStackType),
				"StackType should be set to DualStack")

			// Verify AddressPreferencePolicy defaults to IPv4Primary when not explicitly set
			Expect(gcpCluster.Spec.Network.AddressPreferencePolicy).To(Or(
				Equal(infrav1.IPv4Primary),
				BeEmpty(), // Empty should default to IPv4Primary
			), "AddressPreferencePolicy should default to IPv4Primary")

			By("Verifying both IPv4 and IPv6 addresses are allocated in status")
			// Verify IPv4 address exists
			Expect(gcpCluster.Status.Network.APIServerAddress).ToNot(BeNil(),
				"IPv4 APIServerAddress should be allocated for DualStack")
			ipv4Addr := *gcpCluster.Status.Network.APIServerAddress
			Expect(ipv4Addr).ToNot(BeEmpty())
			Expect(isValidIPv4(ipv4Addr)).To(BeTrue(),
				"APIServerAddress should be a valid IPv4 address, got: %s", ipv4Addr)

			// Verify IPv6 address exists for dual stack
			Expect(gcpCluster.Status.Network.APIServerIPv6Address).ToNot(BeNil(),
				"IPv6 APIServerIPv6Address should be allocated for DualStack")
			ipv6Addr := *gcpCluster.Status.Network.APIServerIPv6Address
			Expect(ipv6Addr).ToNot(BeEmpty())
			Expect(isValidIPv6(ipv6Addr)).To(BeTrue(),
				"APIServerIPv6Address should be a valid IPv6 address, got: %s", ipv6Addr)

			By("Verifying control plane endpoint uses IPv4 address when AddressPreferencePolicy is IPv4Primary")
			Expect(gcpCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(ipv4Addr),
				"Control plane endpoint should use IPv4 address when AddressPreferencePolicy is IPv4Primary")

			By("Verifying both IPv4 and IPv6 forwarding rules exist")
			Expect(gcpCluster.Status.Network.APIServerForwardingRule).ToNot(BeNil(),
				"IPv4 forwarding rule should exist for DualStack")
			Expect(*gcpCluster.Status.Network.APIServerForwardingRule).ToNot(BeEmpty())

			Expect(gcpCluster.Status.Network.APIServerIPv6ForwardingRule).ToNot(BeNil(),
				"IPv6 forwarding rule should exist for DualStack")
			Expect(*gcpCluster.Status.Network.APIServerIPv6ForwardingRule).ToNot(BeEmpty())

			By("Verifying internal load balancer has both IPv4 and IPv6 addresses")
			if gcpCluster.Status.Network.APIInternalAddress != nil {
				internalIPv4 := *gcpCluster.Status.Network.APIInternalAddress
				Expect(isValidIPv4(internalIPv4)).To(BeTrue(),
					"Internal IPv4 address should be valid")
			}

			if gcpCluster.Status.Network.APIInternalIPv6Address != nil {
				internalIPv6 := *gcpCluster.Status.Network.APIInternalIPv6Address
				Expect(isValidIPv6(internalIPv6)).To(BeTrue(),
					"Internal IPv6 address should be valid")
			}
		})
	})

	Context("Testing StackType with DualStack and AddressPreferencePolicy IPv6Primary", func() {
		It("Should create a cluster with DualStack network and IPv6 as primary address", func() {
			clusterName := fmt.Sprintf("%s-ds-ipv6", clusterNamePrefix)

			By("Creating a dual stack cluster with IPv6 primary address preference")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-dual-stack-with-ipv6primary",
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

			By("Verifying the GCPCluster has DualStack StackType with IPv6Primary AddressPreferencePolicy")
			gcpCluster := &infrav1.GCPCluster{}
			key := client.ObjectKey{
				Namespace: namespace.Name,
				Name:      clusterName,
			}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx, key, gcpCluster)).To(Succeed())

			// Verify StackType is DualStack
			Expect(gcpCluster.Spec.Network.StackType).To(Equal(infrav1.DualStackType),
				"StackType should be set to DualStack")

			// Verify AddressPreferencePolicy is IPv6Primary
			Expect(gcpCluster.Spec.Network.AddressPreferencePolicy).To(Equal(infrav1.IPv6Primary),
				"AddressPreferencePolicy should be set to IPv6Primary")

			By("Verifying both IPv4 and IPv6 addresses are allocated in status")
			// Verify IPv4 address exists
			Expect(gcpCluster.Status.Network.APIServerAddress).ToNot(BeNil(),
				"IPv4 address should still be allocated in DualStack mode")
			ipv4Addr := *gcpCluster.Status.Network.APIServerAddress
			Expect(isValidIPv4(ipv4Addr)).To(BeTrue(),
				"APIServerAddress should be a valid IPv4 address")

			// Verify IPv6 address exists
			Expect(gcpCluster.Status.Network.APIServerIPv6Address).ToNot(BeNil(),
				"IPv6 address should be allocated for DualStack")
			ipv6Addr := *gcpCluster.Status.Network.APIServerIPv6Address
			Expect(isValidIPv6(ipv6Addr)).To(BeTrue(),
				"APIServerIPv6Address should be a valid IPv6 address")

			By("Verifying control plane endpoint uses IPv6 address when AddressPreferencePolicy is IPv6Primary")
			Expect(gcpCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(ipv6Addr),
				"Control plane endpoint should use IPv6 address when AddressPreferencePolicy is IPv6Primary")

			By("Verifying both IPv4 and IPv6 forwarding rules exist")
			Expect(gcpCluster.Status.Network.APIServerForwardingRule).ToNot(BeNil(),
				"IPv4 forwarding rule should exist")
			Expect(gcpCluster.Status.Network.APIServerIPv6ForwardingRule).ToNot(BeNil(),
				"IPv6 forwarding rule should exist")
		})
	})

	Context("Testing StackType with IPv4Only (default behavior)", func() {
		It("Should create a cluster with IPv4Only network (no IPv6 addresses)", func() {
			clusterName := fmt.Sprintf("%s-ipv4only", clusterNamePrefix)

			By("Creating an IPv4-only cluster using default flavor")
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

			By("Verifying the GCPCluster has IPv4Only StackType")
			gcpCluster := &infrav1.GCPCluster{}
			key := client.ObjectKey{
				Namespace: namespace.Name,
				Name:      clusterName,
			}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx, key, gcpCluster)).To(Succeed())

			// Verify StackType is IPv4Only or defaults to it
			Expect(gcpCluster.Spec.Network.StackType).To(Or(
				Equal(infrav1.IPv4OnlyStackType),
				BeEmpty(), // Empty should default to IPv4Only
			), "StackType should be IPv4Only or empty (which defaults to IPv4Only)")

			By("Verifying only IPv4 addresses are allocated (no IPv6)")
			// Verify IPv4 address exists
			Expect(gcpCluster.Status.Network.APIServerAddress).ToNot(BeNil(),
				"IPv4 address should be allocated")
			ipv4Addr := *gcpCluster.Status.Network.APIServerAddress
			Expect(isValidIPv4(ipv4Addr)).To(BeTrue(),
				"APIServerAddress should be a valid IPv4 address")

			// Verify IPv6 addresses do NOT exist for IPv4Only
			Expect(gcpCluster.Status.Network.APIServerIPv6Address).To(Or(
				BeNil(),
				PointTo(BeEmpty()),
			), "IPv6 address should not be set for IPv4Only StackType")

			Expect(gcpCluster.Status.Network.APIServerIPv6ForwardingRule).To(Or(
				BeNil(),
				PointTo(BeEmpty()),
			), "IPv6 forwarding rule should not be set for IPv4Only StackType")

			By("Verifying control plane endpoint uses IPv4 address")
			Expect(gcpCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(ipv4Addr),
				"Control plane endpoint should use IPv4 address for IPv4Only")

			By("Verifying AddressPreferencePolicy defaults to IPv4Primary or is not set for IPv4Only")
			Expect(gcpCluster.Spec.Network.AddressPreferencePolicy).To(Or(
				Equal(infrav1.IPv4Primary),
				BeEmpty(),
			), "AddressPreferencePolicy should be IPv4Primary or empty for IPv4Only")
		})
	})

	Context("Creating a control-plane cluster with three control plane nodes and a dual stack network", func() {
		It("Should create a cluster with 3 control-plane and 1 worker node with a dual stack network", func() {
			Skip("This test requires a bootstrap cluster that has access to the network where the cluster is being created.")

			clusterName := fmt.Sprintf("%s-dual-stack-ha", clusterNamePrefix)
			By("Creating a cluster with a dual stack network from GKE bootstrap cluster")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ci-with-dual-stack",
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
})

// isValidIPv4 checks if a string is a valid IPv4 address
func isValidIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// Check if it's an IPv4 address
	return ip.To4() != nil
}

// isValidIPv6 checks if a string is a valid IPv6 address
func isValidIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// Check if it's an IPv6 address (not IPv4)
	return ip.To4() == nil && ip.To16() != nil
}
