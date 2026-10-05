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
```

In this case, the flavor `ci-with-internal-lb` is a reference to the template `cluster-template-ci-with-internal-lb.yaml` which is available in `./test/e2e/data/infrastructure-gcp/cluster-template-ci-with-internal-lb.yaml`.

## Confirm the presubmit runs the test

Adding a Ginkgo spec does not ensure that a CAPG presubmit executes it. The `test-e2e-run` target in the [Makefile](../../../../Makefile) passes `GINKGO_FOCUS` to Ginkgo's `--focus` flag. Prow jobs can set `GINKGO_FOCUS` themselves, overriding the Makefile default. For example, the unmanaged E2E presubmit currently focuses on `Workload cluster creation`; a separate `Autoscaling from zero` context would not match that focus.

Before relying on a new E2E test as PR coverage:

1. Find the relevant job in the [main-branch Prow presubmit configuration](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes-sigs/cluster-api-provider-gcp/cluster-api-provider-gcp-presubmits-main.yaml), and check its `GINKGO_FOCUS` value. The [CI jobs guide](jobs.md) links to the job results.
2. Check that the focus matches the full Ginkgo spec name, including its parent `Describe` and `Context` text. A passing job may have selected other specs while skipping the new one.
3. After the presubmit runs, confirm that the new spec appears in its log or JUnit report. Name the job and the executed spec in the PR's testing notes.

If the new spec is excluded, discuss whether it belongs in an existing scenario or whether the job focus needs an update in `kubernetes/test-infra`. Choose coverage that tests the intended behavior without adding unnecessary cluster provisioning to every PR run.
