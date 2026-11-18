# Submitting a Change

### Submitting PRs and testing

Pull requests and issues are highly encouraged!
If you're interested in submitting PRs to the project, please be sure to run some initial checks prior to submission:

Do make sure to set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable with the path to your JSON file. Check out the this [doc](https://cloud.google.com/docs/authentication/production) to generate the credential.

```shell
make lint # Runs a suite of quick scripts to check code structure
make test # Runs tests on the Go code
```

#### Executing unit tests

`make test` executes the project's unit tests. These tests need no Kubernetes cluster or GCP access. Some of them
(the `envtest` suites) start a local API server and `etcd`, whose binaries `make test` downloads on first use.

#### Test Frameworks

When adding new tests, which should be done for the vast majority of changes, we have standardised on some specific frameworks:

* Unit Tests: These are plain `go test` tests with `gomega` accessed via `NewWithT(t)`
  * This is in preference to `testify`, which will cause linting to fail if used.
  * **Note:** When using `g := NewWithT(t)` you should always use the 'nearest' `t`, rather than anything global. Definitely do not use the package-level `Expect`.
* Tests that require `envtest`: These are written with Ginkgo and are often useful for testing CRD and CEL validation
* End-To-End Tests: These are written with Ginkgo.
