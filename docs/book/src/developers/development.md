# Developing Cluster API Provider GCP

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.18.
2. Install [jq][jq]
   - `brew install jq` on macOS.
   - `sudo apt install jq` on Windows + WSL2.
   - `sudo apt install jq` on Ubuntu Linux.
3. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on macOS.
   - `sudo apt install gettext` on Windows + WSL2.
   - `sudo apt install gettext` on Ubuntu Linux.
4. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.14.0`.
5. Install [Kustomize][kustomize]
   - `brew install kustomize` on macOS.
   - [install instructions](https://kubectl.docs.kubernetes.io/installation/kustomize/) on Windows + WSL2, Linux and macOS.
6. Install Python 3.x, if neither is already installed.
7. Install make.
   - `brew install make` on MacOS.
   - `sudo apt install make` on Windows + WSL2.
   - `sudo apt install make` on Linux.
8. Install [timeout][timeout]
   - `brew install coreutils` on macOS.

When developing on Windows, it is suggested to set up the project on Windows + WSL2 and the file should be checked out on as wsl file system for better results.

### Get the source

```shell
git clone https://github.com/kubernetes-sigs/cluster-api-provider-gcp
cd cluster-api-provider-gcp
```

### Get familiar with basic concepts

This provider is modeled after the upstream Cluster API project. To get familiar
with Cluster API resources, concepts and conventions ([such as CAPI and CAPG](https://cluster-api.sigs.k8s.io/reference/glossary.html#c)), refer to the [Cluster API Book](https://cluster-api.sigs.k8s.io/).

### Dev manifest files

Part of running cluster-api-provider-gcp is generating manifests to run.
Generating dev manifests allows you to test dev images instead of the default
releases.

### Dev images

#### Container registry

Any public container registry can be leveraged for storing cluster-api-provider-gcp container images.

## CAPG Node images

In order to deploy a workload cluster you will need to build the node images to use, for that you can reference the [image-builder](https://github.com/kubernetes-sigs/image-builder)
project, also you can read the [image-builder book](https://image-builder.sigs.k8s.io/)

Please refer to the image-builder documentation in order to get the latest requirements to build the node images.

To build the node images for GCP: [https://image-builder.sigs.k8s.io/capi/providers/gcp.html](https://image-builder.sigs.k8s.io/capi/providers/gcp.html)

## Developing

Change some code!

### Modules and Dependencies

This repository uses [Go Modules](https://github.com/golang/go/wiki/Modules) to track vendor dependencies.

To pin a new dependency:

- Run `go get <repository>@<version>`
- (Optional) Add a replace statement in `go.mod`

Makefile targets and scripts are offered to work with go modules:

- `make verify-modules` checks whether go modules are out of date.
- `make modules` runs `go mod tidy` to ensure proper vendoring.
- `hack/ensure-go.sh` checks that the Go version and environment variables are properly set.

### Setting up the environment

Your environment must have the GCP credentials, check [Authentication Getting Started](https://cloud.google.com/docs/authentication/getting-started)

### Tilt Requirements

Install [Tilt][tilt]:

- `brew install tilt-dev/tap/tilt` on macOS or Linux
- `scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket` & `scoop install tilt` on Windows

After the installation is done, verify that you have installed it correctly with: `tilt version`

Install [Helm](https://helm.sh/docs/intro/install/):

- `brew install helm` on MacOS
- `choco install kubernetes-helm` on Windows
- [Install instructions](https://helm.sh/docs/intro/install/#from-source-linux-macos) for Linux

As the project lacks a lot of feature for windows, it would be suggested to follow the above steps on Windows + WSL2
rather than Windows.

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPG in a local kind cluster. The main difference is the number of components you will build from source and the scope of the changes you'd like to make. If you only want to make changes in CAPG, then follow [CAPG instructions](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/docs/book/src/developers/development.md#tilt-for-dev-in-capg). This will save you from having to build all of the images for CAPI, which can take a while. If the scope of your development will span both CAPG and CAPI, then follow the [CAPI and CAPG instructions](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/docs/book/src/developers/development.md#tilt-for-dev-in-both-capg-and-capi).

#### Tilt for dev in CAPG

If you want to develop in CAPG and get a local development cluster working quickly, this is the path for you.

From the root of the CAPG repository, run the following to generate a `tilt-settings.json` file with your GCP
service account credentials:

```shell
$ cat <<EOF > tilt-settings.json
{
  "kustomize_substitutions": {
      "GCP_B64ENCODED_CREDENTIALS": "$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"
  }
}
EOF
```

Set the following environment variables with the appropriate values for your environment:

```shell
$ export GCP_REGION="<GCP_REGION>" \
$ export GCP_PROJECT="<GCP_PROJECT>" \
$ export CONTROL_PLANE_MACHINE_COUNT=1 \
$ export WORKER_MACHINE_COUNT=1 \
# Make sure to use same kubernetes version here as building the GCE image
$ export KUBERNETES_VERSION=1.23.3 \
$ export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2 \
$ export GCP_NODE_MACHINE_TYPE=n1-standard-2 \
$ export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default> \
$ export CLUSTER_NAME="<CLUSTER_NAME>" \
```

To build a kind cluster and start Tilt, just run:

```shell
make tilt-up
```

Alternatively, you can also run:

```shell
./scripts/setup-dev-enviroment.sh
```

It will setup the network, if you already setup the network you can skip this step for that just run:

```shell
./scripts/setup-dev-enviroment.sh --skip-init-network
```

By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/core/tilt.html?highlight=tilt#create-a-tilt-settings-file).

Once your kind management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the kind cluster built by the command above, just run:

```shell
make kind-reset
```

And if you need to cleanup the network setup you can run:

```shell
./scripts/setup-dev-enviroment.sh --clean-network
```

#### Tilt for dev in both CAPG and CAPI

If you want to develop in both CAPI and CAPG at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo. The instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use `kind` to deploy the cluster api management components.

> you may wish to checkout out the correct version of CAPI to match the [version used in CAPG][go.mod]

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.json` file will point back to the `cluster-api-provider-gcp` repository directory. Any changes you make to the source code in `cluster-api` or `cluster-api-provider-gcp` repositories will automatically redeployed to the `kind` cluster.

After you have cloned both repositories, your folder structure should look like:

```tree
|-- src/cluster-api-provider-gcp
|-- src/cluster-api (run `tilt up` here)
```

After configuring the environment variables, run the following to generate your `tilt-settings.json` file:

```shell
cat <<EOF > tilt-settings.json
{
  "default_registry": "${REGISTRY}",
  "provider_repos": ["../cluster-api-provider-gcp"],
  "enable_providers": ["gcp", "docker", "kubeadm-bootstrap", "kubeadm-control-plane"],
  "kustomize_substitutions": {
      "GCP_B64ENCODED_CREDENTIALS": "$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"
  }
}
EOF
```

> `$REGISTRY` should be in the format `docker.io/<dockerhub-username>`

The cluster-api management components that are deployed are configured at the `/config` folder of each repository respectively. Making changes to those files will trigger a redeploy of the management cluster components.

#### Debugging

If you would like to debug CAPG you can run the provider with [delve](https://github.com/go-delve/delve), a Go debugger tool. This will then allow you to attach to delve and troubleshoot the processes.

To do this you need to use the debug configuration in tilt-settings.json. Full details of the options can be seen [here](https://cluster-api.sigs.k8s.io/developer/tilt.html).

An example tilt-settings.json:

```shell
{
  "default_registry": "gcr.io/your-project-name-her",
  "provider_repos": ["../cluster-api-provider-gcp"],
  "enable_providers": ["gcp", "kubeadm-bootstrap", "kubeadm-control-plane"],
  "debug": {
    "gcp": {
      "continue": true,
      "port": 30000,
      "profiler_port": 40000,
      "metrics_port": 40001
    }
  },
  "kustomize_substitutions": {
      "GCP_B64ENCODED_CREDENTIALS": "$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"
  }
}
```

Once you have run tilt (see section below) you will be able to connect to the running instance of delve.

For vscode, you can use the a launch configuration like this:

```shell
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Core CAPI Controller GCP",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "remotePath": "",
      "port": 30000,
      "host": "127.0.0.1",
      "showLog": true,
      "trace": "log",
      "logOutput": "rpc"
    }
  ]
}
```

Create a new configuration and add it to the "Debug" menu to configure debugging in GoLand/IntelliJ following [these instructions](https://www.jetbrains.com/help/go/attach-to-running-go-processes-with-debugger.html#step-3-create-the-remote-run-debug-configuration-on-the-client-computer).

Alternatively, you may use delve straight from the CLI by executing a command like this:

```shell
delve -a tcp://localhost:30000
```

#### Deploying a workload cluster

After your kind management cluster is up and running with Tilt, ensure you have all the environment variables set as
described in [Tilt for dev in CAPG](#tilt-for-dev-in-capg), and deploy a workload cluster with the following:

```shell
make create-workload-cluster
```

To delete the cluster:

```shell
make delete-workload-cluster
```

### Submitting PRs and testing

Pull requests and issues are highly encouraged!
If you're interested in submitting PRs to the project, please be sure to run some initial checks prior to submission:

Do make sure to set the `GOOGLE_APPLICATION_CREDENTIALS` environment variable with the path to your JSON file. Check out the this [doc](https://cloud.google.com/docs/authentication/production) to generate the credential.

```shell
make lint # Runs a suite of quick scripts to check code structure
make test # Runs tests on the Go code
```

#### Executing unit tests

`make test` executes the project's unit tests. These tests do not stand up a
Kubernetes cluster, nor do they have external dependencies.

[go]: https://golang.org/doc/install
[tilt]: https://docs.tilt.dev/install.html
[jq]: https://stedolan.github.io/jq/download/
[gettext]: https://www.gnu.org/software/gettext/
[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/master/go.mod
[kind]: https://sigs.k8s.io/kind
[kustomize]: https://github.com/kubernetes-sigs/kustomize
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
