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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

const (
	retryableOperationInterval = 3 * time.Second
	retryableOperationTimeout  = 3 * time.Minute
)

// ApplyManagedClusterTemplateAndWaitInput is the input type for ApplyManagedClusterTemplateAndWait.
type ApplyManagedClusterTemplateAndWaitInput struct {
	ClusterProxy                   framework.ClusterProxy
	ConfigCluster                  clusterctl.ConfigClusterInput
	WaitForClusterIntervals        []interface{}
	WaitForControlPlaneIntervals   []interface{}
	WaitForMachinePools            []interface{}
	Args                           []string // extra args to be used during `kubectl apply`
	PreWaitForCluster              func()
	PostMachinesProvisioned        func()
	WaitForControlPlaneInitialized Waiter
}

// Waiter is a function that runs and waits for a long-running operation to finish and updates the result.
type Waiter func(ctx context.Context, input ApplyManagedClusterTemplateAndWaitInput, result *ApplyManagedClusterTemplateAndWaitResult)

// ApplyManagedClusterTemplateAndWaitResult is the output type for ApplyClusterTemplateAndWait.
type ApplyManagedClusterTemplateAndWaitResult struct {
	ClusterClass *clusterv1.ClusterClass
	Cluster      *clusterv1.Cluster
	ControlPlane *infrav1exp.GCPManagedControlPlane
	MachinePools []*expv1.MachinePool
}

// ApplyManagedClusterTemplateAndWait gets a managed cluster template using clusterctl, and waits for the cluster to be ready.
// Important! this method assumes the cluster uses a GCPManagedControlPlane and MachinePools.
func ApplyManagedClusterTemplateAndWait(ctx context.Context, input ApplyManagedClusterTemplateAndWaitInput, result *ApplyManagedClusterTemplateAndWaitResult) {
	setDefaults(&input)
	Expect(ctx).NotTo(BeNil(), "ctx is required for ApplyManagedClusterTemplateAndWait")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling ApplyManagedClusterTemplateAndWait")
	Expect(result).ToNot(BeNil(), "Invalid argument. result can't be nil when calling ApplyManagedClusterTemplateAndWait")
	Expect(input.ConfigCluster.Flavor).ToNot(BeEmpty(), "Invalid argument. input.ConfigCluster.Flavor can't be empty")
	Expect(input.ConfigCluster.ControlPlaneMachineCount).ToNot(BeNil())
	Expect(input.ConfigCluster.WorkerMachineCount).ToNot(BeNil())

	Byf("Creating the GKE workload cluster with name %q using the %q template (Kubernetes %s)",
		input.ConfigCluster.ClusterName, input.ConfigCluster.Flavor, input.ConfigCluster.KubernetesVersion)

	By("Getting the cluster template yaml")
	workloadClusterTemplate := clusterctl.ConfigCluster(ctx, clusterctl.ConfigClusterInput{
		// pass reference to the management cluster hosting this test
		KubeconfigPath: input.ConfigCluster.KubeconfigPath,
		// pass the clusterctl config file that points to the local provider repository created for this test,
		ClusterctlConfigPath: input.ConfigCluster.ClusterctlConfigPath,
		// select template
		Flavor: input.ConfigCluster.Flavor,
		// define template variables
		Namespace:                input.ConfigCluster.Namespace,
		ClusterName:              input.ConfigCluster.ClusterName,
		KubernetesVersion:        input.ConfigCluster.KubernetesVersion,
		ControlPlaneMachineCount: input.ConfigCluster.ControlPlaneMachineCount,
		WorkerMachineCount:       input.ConfigCluster.WorkerMachineCount,
		InfrastructureProvider:   input.ConfigCluster.InfrastructureProvider,
		// setup clusterctl logs folder
		LogFolder:           input.ConfigCluster.LogFolder,
		ClusterctlVariables: input.ConfigCluster.ClusterctlVariables,
	})
	Expect(workloadClusterTemplate).ToNot(BeNil(), "Failed to get the cluster template")

	By("Applying the cluster template yaml to the cluster")
	Eventually(func() error {
		return input.ClusterProxy.Apply(ctx, workloadClusterTemplate, input.Args...)
	}, 10*time.Second).Should(Succeed(), "Failed to apply the cluster template")

	// Once we applied the cluster template we can run PreWaitForCluster.
	// Note: This can e.g. be used to verify the BeforeClusterCreate lifecycle hook is executed
	// and blocking correctly.
	if input.PreWaitForCluster != nil {
		By("Calling PreWaitForCluster")
		input.PreWaitForCluster()
	}

	By("Waiting for the cluster infrastructure to be provisioned")
	result.Cluster = framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		Getter:    input.ClusterProxy.GetClient(),
		Namespace: input.ConfigCluster.Namespace,
		Name:      input.ConfigCluster.ClusterName,
	}, input.WaitForClusterIntervals...)

	By("Waiting for managed control plane to be initialized")
	input.WaitForControlPlaneInitialized(ctx, input, result)

	By("Waiting for the machine pools to be provisioned")
	result.MachinePools = framework.DiscoveryAndWaitForMachinePools(ctx, framework.DiscoveryAndWaitForMachinePoolsInput{
		Getter:  input.ClusterProxy.GetClient(),
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForMachinePools...)

	if input.PostMachinesProvisioned != nil {
		By("Calling PostMachinesProvisioned")
		input.PostMachinesProvisioned()
	}
}

type ManagedControlPlaneResult struct {
	clusterctl.ApplyClusterTemplateAndWaitResult

	ManagedControlPlane *infrav1exp.GCPManagedControlPlane
}

// DiscoveryAndWaitFoManagedControlPlaneInitializedInput is the input type for DiscoveryAndWaitForManagedControlPlaneInitialized.
type DiscoveryAndWaitForManagedControlPlaneInitializedInput struct {
	Lister  framework.Lister
	Cluster *clusterv1.Cluster
}

// DiscoveryAndWaitForManagedControlPlaneInitialized discovers the KubeadmControlPlane object attached to a cluster and waits for it to be initialized.
func DiscoveryAndWaitForManagedControlPlaneInitialized(ctx context.Context, input DiscoveryAndWaitForManagedControlPlaneInitializedInput, intervals ...interface{}) *infrav1exp.GCPManagedControlPlane {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForManagedControlPlaneInitialized")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForManagedControlPlaneInitialized")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoveryAndWaitForManagedControlPlaneInitialized")

	By("Getting GCPManagedControlPlane control plane")

	var controlPlane *infrav1exp.GCPManagedControlPlane
	Eventually(func(g Gomega) {
		controlPlane = GetManagedControlPlaneByCluster(ctx, GetManagedControlPlaneByClusterInput{
			Lister:      input.Lister,
			ClusterName: input.Cluster.Name,
			Namespace:   input.Cluster.Namespace,
		})
		g.Expect(controlPlane).ToNot(BeNil())
	}, "10s", "1s").Should(Succeed(), "Couldn't get the control plane for the cluster %s", klog.KObj(input.Cluster))

	return controlPlane
}

// GetManagedontrolPlaneByClusterInput is the input for GetManagedControlPlaneByCluster.
type GetManagedControlPlaneByClusterInput struct {
	Lister      framework.Lister
	ClusterName string
	Namespace   string
}

// GetManagedControlPlaneByCluster returns the GCPManagedControlPlane objects for a cluster.
func GetManagedControlPlaneByCluster(ctx context.Context, input GetManagedControlPlaneByClusterInput) *infrav1exp.GCPManagedControlPlane {
	opts := []client.ListOption{
		client.InNamespace(input.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: input.ClusterName,
		},
	}

	controlPlaneList := &infrav1exp.GCPManagedControlPlaneList{}
	Eventually(func() error {
		return input.Lister.List(ctx, controlPlaneList, opts...)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to list GCPManagedControlPlane object for Cluster %s", klog.KRef(input.Namespace, input.ClusterName))
	Expect(len(controlPlaneList.Items)).ToNot(BeNumerically(">", 1), "Cluster %s should not have more than 1 GCPManagedControlPlane object", klog.KRef(input.Namespace, input.ClusterName))
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

func setDefaults(input *ApplyManagedClusterTemplateAndWaitInput) {
	if input.WaitForControlPlaneInitialized == nil {
		input.WaitForControlPlaneInitialized = func(ctx context.Context, input ApplyManagedClusterTemplateAndWaitInput, result *ApplyManagedClusterTemplateAndWaitResult) {
			result.ControlPlane = DiscoveryAndWaitForManagedControlPlaneInitialized(ctx, DiscoveryAndWaitForManagedControlPlaneInitializedInput{
				Lister:  input.ClusterProxy.GetClient(),
				Cluster: result.Cluster,
			}, input.WaitForControlPlaneIntervals...)
		}
	}
}
