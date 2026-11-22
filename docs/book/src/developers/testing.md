# Testing in CAPG

Testing for a project like CAPG can be very complicated because we have to test integrations across multiple systems,
including GCP and CAPI, so it can sometimes be slightly overwhelming. This page lays out the different testing
options that are available within CAPG and some details on how they work to aid in choosing how to test any changes you
want to make.

## Choosing a Test Layer

Testing in CAPG, as in any application, is a series of layers and choosing the right layer to test a change at is one
of the most important decisions to be made when designing tests. The key idea to keep in mind, is to always pick the
smallest layer that proves the behaviour you need. Not every change needs big, expensive end-to-end tests, when the
behaviour you're trying to validate is small and well-scoped.

In the next sections we'll explain the different kinds of tests available and how they work, this should aid you
in choosing the kind of tests you want to write.

## Unit Tests

Unit tests exist to very cheaply verify behaviour by testing a small part of the code in isolation. These are by far
the most common kinds of tests, as if you design your changes well you can isolate the change to a very small area of
the code and test it extensively.

When designing unit tests we use `go test` and `gomega` for assertions, accessed via `NewWithT(t)`. This is in
preference to `testify`, which will cause linting to fail if used. When using `g := NewWithT(t)` you should always use
the 'nearest' `t`, rather than anything global. Definitely do not use the package level `Expect`.

These can be executed with `make test` (this will also run the integration tests). In CI these run as `pull-cluster-api-provider-gcp-test` and you can find out more about this CI target in the
[CI Jobs](./orientation.md#ci-jobs) section of our Repo Orientation page.

## Integration Tests

Integration tests are the next level up from Unit tests where you want to orchestrate multiple pieces of code together
and prove they integrate well. In CAPG this often (though not always) means using `envtest`. `envtest` starts a local
API server and `etcd`, whose binaries `make test` downloads on first use. These are often useful for testing CEL
assertions or webhooks that don't require the entire application but do need more of the Kubernetes infrastructure to
be running. These are also run via `make test` and run in CI under the same job as the Unit Tests above.

## Test Coverage & Reporting

We have two bespoke `make` targets that are designed for the purposes of coverage and test reporting specifically. The
first, `make test-cover` runs `make test` with a coverage profile and then renders a report to show what parts of the
code are covered by tests and which are not.

`make test-junit` is our specialized reporting target, it runs the same tests as `make test` but it will also generate a
report in the JUnit format.

## End-to-End Tests

E2E tests verify a complete, real-world workflow ensuring that all parts of the system work together as expected. If you
are introducing a new feature that interconnects with other parts of the software, you will likely be required to add a
verification step for this functionality with a new E2E scenario. Unless it is already covered by existing test suites.

The end-to-end tests themselves cover self-managed and GKE clusters as well as upstream CAPI specs and Kubernetes
Upgrades. To do this the [`ci-e2e.sh`](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/scripts/ci-e2e.sh)
provisions a new shared VPC with the correct firewall rules, a Cloud Router and NAT. Though CAPG can set these up
itself, in the E2E tests this is externalised so that each cluster can share the same network and tests can run
isolated, but concurrently. Once this is complete, the E2E testing code creates a KinD cluster called `capg-e2e` and
runs the test specifications against that KinD cluster.

The tests also clean up themselves by removing resources that are named `capg-e2e`, so our recommendation is to run
these in a dedicated GCP project. This way any deletes do not conflict with existing resources. You can skip
cleanup via setting the `SKIP_CLEANUP` flag to any value but this will leave behind (potentially costly) resources.
You will need to then clean these up by hand after the fact, but you can use the `capg-e2e` name to find the necessary
resources.

In CI our GCP projects come from [Boskos](https://github.com/kubernetes-sigs/boskos), whereas if you run them locally
you'll need to provide your own GCP project. For more information on how these run in CI see the
`pull-cluster-api-provider-gcp-e2e-test` & `pull-cluster-api-provider-gcp-e2e-test-gke` rows in the
[CI Jobs Section](./orientation.md#ci-jobs) of our Repository Orientation.

**Note:** In CI two `internal-load-balancer` specs are skipped, this is deliberate due to issues with network routing
between our KinD management cluster and the workload clusters.

### Running End-To-End Tests Locally

We do not recommend running the entire end-to-end suite locally as it will spin up many clusters in your GCP account
and so could be expensive, if you need to do this do it via raising a PR and have it run in CI. However, if you want to
run single scenarios while developing this should be of limited cost.

To do this set up your environment thus:

```shell
export GCP_PROJECT=<GCP_PROJECT>
export GCP_REGION=<GCP_REGION>
```

Where GCP_PROJECT and GCP_REGION match the project you want to run the tests in. Also ensure that
`GOOGLE_APPLICATION_CREDENTIALS` is set to point to a valid credential file.

If you want to create a zonal cluster (using a zonal flavor) you can specify the zone by exporting `GCP_ZONE` before
running those suites. This zone must be inside `GCP_REGION`. If you run the suites without the script (`ci-e2e.sh`) you
will need to set `GCP_ZONE` manually, whereas the script sets `GCP_ZONE` to `GCP_REGION` with a suffix of `-a`.

Then run:

```shell
GINKGO_FOCUS=<SPEC_TEXT> ./scripts/ci-e2e.sh --flavor=<FLAVOR>
```
Where SPEC_TEXT is a regular expression that should match with the text of the Ginkgo specification you want to
run and FLAVOR should match the "flavor" of test you want to run (unmanaged, gke or upgrade). Beware that not all
specifications run for all flavors so check the specification before running it.

As GINKGO_FOCUS matches by regular expression against the Ginkgo spec text,  make sure to be specific if you don't want
multiple specs to run. A good check is the Ginkgo log line `Ran N of 19 Specs`, which will appear early in the test run
if you want to make sure your selection has only matched a small number of specs.

**Note:** The script will switch your active `gcloud` account so use a separate `CLOUDSDK_CONFIG`. It will also, upon
completion, leave your `kubectl` context pointing at `kind-capg-e2e` which no longer exists. Switch back to a correct
context by using `kubectl config use-context`.

### Adding An End-to-End Test

To add an End-To-End test you first need to decide (or define) a cluster template, and then add a test case that
exercises it. The following shows how to achieve each of those steps.

#### Adding a Cluster Template

The test suite will provision a cluster based on a pre-defined yaml template (stored in `./test/e2e/data`) which is then sourced in `./test/e2e/config/gcp-ci.yaml`. New cluster definitions for E2E tests have to be added and sourced before being available to use in the E2E workflow.

#### Adding A New Test Case

When the template is available, you can reference it as a flavor in Go. For example, adding a new test for self-managed cluster provisioning would look like the following:

```golang
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
```

In this case, the flavor `ci-with-creds` is a reference to the template
[`cluster-template-ci-with-creds.yaml`](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/test/e2e/data/infrastructure-gcp/cluster-template-ci-with-creds.yaml).

## Conformance Tests

Conformance tests are to ensure that CAPG is compatible with Kubernetes as Kubernetes itself changes, as such we
run two different variants, one against the version of Kubernetes targeted by CAPG, and one against the latest build
of the `main` `kubernetes/kubernetes` branch.

These tests should only be run via CI (due to their prohibitive costs and runtime). In CI they run as `pull-cluster-api-provider-gcp-conformance` and `pull-cluster-api-provider-gcp-conformance-ci-artifacts`, for more
information, including how to manually trigger these tests on a PR, see the [CI Jobs](./orientation.md#ci-jobs) section
of our Repo Orientation page.