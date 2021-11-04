# Jobs

This document provides an overview of our jobs running via Prow and Github actions.

## Builds and tests running on the default branch

<aside class="note">

<h1> Note </h1>

To see which test jobs execute which e2e tests, you can click on the links which lead to the respective test overviews in testgrid.

</aside>

### Legend

    🟢 REQUIRED - Jobs that have to run successfully to get the PR merged.

### Presubmits

   Prow Presubmits:

-  🟢[pull-cluster-api-provider-gcp-test] `./scripts/ci-test.sh`
-  🟢[pull-cluster-api-provider-gcp-build] `../scripts/ci-build.sh`
-  🟢[pull-cluster-api-provider-gcp-make] `runner.sh` `./scripts/ci-make.sh`
-  🟢[pull-cluster-api-provider-gcp-e2e-test]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-e2e.sh`
- [pull-cluster-api-provider-gcp-conformance-ci-artifacts]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh --use-ci-artifacts`
- [pull-cluster-api-provider-gcp-conformance]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh`
- [pull-cluster-api-provider-gcp-capi-e2e]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" GINKGO_FOCUS="Cluster API E2E tests" ./scripts/ci-e2e.sh`
- [pull-cluster-api-provider-gcp-test-release-0-4] `./scripts/ci-test.sh`
- [pull-cluster-api-provider-gcp-build-release-0-4] `./scripts/ci-build.sh`
- [pull-cluster-api-provider-gcp-make-release-0-4] `runner.sh` `./scripts/ci-make.sh`
- [pull-cluster-api-provider-gcp-e2e-test-release-0-4]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-e2e.sh`
- [pull-cluster-api-provider-gcp-make-conformance-release-0-4]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh --use-ci-artifacts`

  

  Github Presubmits Workflows:

- Markdown-link-check `find . -name \*.md | xargs -I{} markdown-link-check -c .markdownlinkcheck.json {}`
-  🟢Lint-check `make lint`

### Postsubmits

  Github Postsubmit Workflows:

- Code-coverage-check `make test-cover`

### Periodics

   Prow Periodics:

- [periodic-cluster-api-provider-gcp-build] `runner.sh` `./scripts/ci-build.sh`
- [periodic-cluster-api-provider-gcp-test] `runner.sh` `./scripts/ci-test.sh`
- [periodic-cluster-api-provider-gcp-make-conformance-v1alpha4]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh`
- [periodic-cluster-api-provider-gcp-make-conformance-v1alpha4-k8s-ci-artifacts]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh --use-ci-artifacts`
- [periodic-cluster-api-provider-gcp-conformance-v1alpha4]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh`
- [periodic-cluster-api-provider-gcp-conformance-v1alpha4-k8s-ci-artifacts]
  `"BOSKOS_HOST"="boskos.test-pods.svc.cluster.local" ./scripts/ci-conformance.sh --use-ci-artifacts`

<!-- links -->
[pull-cluster-api-provider-gcp-test]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-test
[pull-cluster-api-provider-gcp-build]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-build
[pull-cluster-api-provider-gcp-make]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-make
[pull-cluster-api-provider-gcp-e2e-test]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-e2e-test
[pull-cluster-api-provider-gcp-conformance-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-conformance-ci-artifacts
[pull-cluster-api-provider-gcp-conformance]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-conformance
[pull-cluster-api-provider-gcp-capi-e2e]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-capi-e2e-test
[pull-cluster-api-provider-gcp-test-release-0-4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-test-release-0-4
[pull-cluster-api-provider-gcp-build-release-0-4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-build-release-0-4
[pull-cluster-api-provider-gcp-make-release-0-4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-make-release-0-4
[pull-cluster-api-provider-gcp-e2e-test-release-0-4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-e2e-test-release-0-4
[pull-cluster-api-provider-gcp-make-conformance-release-0-4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#pr-conformance-release-0-4
[periodic-cluster-api-provider-gcp-build]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#periodic-cluster-api-provider-gcp-build
[periodic-cluster-api-provider-gcp-test]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#periodic-cluster-api-provider-gcp-test
[periodic-cluster-api-provider-gcp-make-conformance-v1alpha4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#capg-conformance-v1alpha4
[periodic-cluster-api-provider-gcp-make-conformance-v1alpha4-k8s-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#capg-conformance-v1alpha4-k8s-master
[periodic-cluster-api-provider-gcp-conformance-v1alpha4]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#capg-conformance-v1alpha4-release-0-4
[periodic-cluster-api-provider-gcp-conformance-v1alpha4-k8s-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-gcp#capg-conformance-v1alpha4-release-0-4-ci-artifacts
