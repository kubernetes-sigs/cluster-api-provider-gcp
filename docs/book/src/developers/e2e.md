# Adding new E2E test

E2E tests verify a complete, real-world workflow ensuring that all parts of the system work together as expected. If you are introducing a new feature that interconnects with other parts of the software, you will likely be required to add a verification step for this functionality with a new E2E scenario (unless it is already covered by existing test suites).

<aside class="note">

<h1>Tip</h1>

You can find all logic and configuration files in the E2E folder of the repository [here](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/test/e2e)

</aside>

## Create a cluster template

The test suite will provision a cluster based on a pre-defined yaml template (stored in `./test/e2e/data`) which is then sourced in `./test/e2e/config/gcp-ci.yaml`. New cluster definitions for E2E tests have to be added and sourced before being available to use in the E2E workflow.

## Add test case

When the template is available, you can reference it as a flavor in Go. For example, adding a new test for self-managed cluster provisioning would look like the following:

```golang
Context("Creating a control-plane cluster with an internal load balancer", func() {
    It("Should create a cluster with 1 control-plane and 1 worker node with an internal load balancer", func() {
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
                KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
                ControlPlaneMachineCount: ptr.To[int64](1),
                WorkerMachineCount:       ptr.To[int64](1),
            },
            WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
            WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
            WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
        }, result)
    })
})
```

In this case, the flavor `ci-with-internal-lb` is a reference to the template `cluster-template-ci-with-internal-lb.yaml` which is available in `./test/e2e/data/infrastructure-gcp/cluster-template-ci-with-internal-lb.yaml`.
