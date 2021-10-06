# Developing Cluster API Provider GCP

## Setting up

### Base requirements

1. Install [go][go]
   - Get the latest patch version for go v1.16.
2. Install [tilt][tilt]
3. Install [jq][jq]
   - `brew install jq` on macOS.
   - `sudo apt install jq` on Windows + WSL2.
   - `sudo apt install jq` on Ubuntu Linux.
4. Install [gettext][gettext] package
   - `brew install gettext && brew link --force gettext` on macOS.
   - `sudo apt install gettext` on Windows + WSL2.
   - `sudo apt install gettext` on Ubuntu Linux.
5. Install [KIND][kind]
   - `GO111MODULE="on" go get sigs.k8s.io/kind@v0.11.1`.
6. Install [Kustomize][kustomize]
   - `brew install kustomize` on macOS.
   - [install instructions](https://kubectl.docs.kubernetes.io/installation/kustomize/) on Windows + WSL2.
   - [install instructions][kustomizelinux] on Linux
7. Install Python 3.x or 2.7.x, if neither is already installed.
8. Install make.
   - `brew install make` on MacOS.
   - `sudo apt install make` on Windows + WSL2.
   - `sudo apt install make` on Linux.
9. Install [timeout][timeout]
   - `brew install coreutils` on macOS.

When developing on Windows, it is suggested to set up the project on Windows + WSL2 and the file should be checked out on as wsl file system for better results.

### Get the source

```shell
go get -d sigs.k8s.io/cluster-api-provider-gcp
cd "$(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-gcp"
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


### Setting up the environment

Your environment must have the GCP credentials, check [Authentication Getting Started](https://cloud.google.com/docs/authentication/getting-started)

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPG in a local kind cluster.

#### Tilt for dev in CAPG

If you want to develop in CAPG and get a local development cluster working quickly, this is the path for you.

From the root of the CAPG repository and after configuring the environment variables, you can run the following to generate your `tilt-settings.json` file:

```shell
$ cat <<EOF > tilt-settings.json
{
  "kustomize_substitutions": {
      "GCP_B64ENCODED_CREDENTIALS": "$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"
  }
}
EOF
```

Need setup some environment variables:

```shell
$ export GCP_REGION="<GCP_REGION>" \
$ export GCP_PROJECT="<GCP_PROJECT>" \
$ export CONTROL_PLANE_MACHINE_COUNT=1 \
$ export WORKER_MACHINE_COUNT=1 \
$ export KUBERNETES_VERSION=1.20.9 \
$ export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2 \
$ export GCP_NODE_MACHINE_TYPE=n1-standard-2 \
$ export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default> \
$ export CLUSTER_NAME="<CLUSTER_NAME>" \
```

To build a kind cluster and start Tilt, just run:

```shell
$ ./scripts/setup-dev-enviroment.sh
```

It will setup the network, if you already setup the network you can skip this step for that just run:

```shell
$ ./scripts/setup-dev-enviroment.sh --skip-init-network
```

By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/tilt.html#create-a-tilt-settingsjson-file).

Once your kind management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the kind cluster built by the command above, just run:

```shell
$ make kind-reset
```

And if you need to cleanup the network setup you can run:

```shell
$ ./scripts/setup-dev-enviroment.sh --clean-network
```

#### Tilt for dev in both CAPG and CAPI

If you want to develop in both CAPI and CAPG at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo.  The instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use `kind` to deploy the cluster api management components.

> you may wish to checkout out the correct version of CAPI to match the [version used in CAPG][go.mod]

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.json` file will point back to the `cluster-api-provider-gcp` repository directory.  Any changes you make to the source code in `cluster-api` or `cluster-api-provider-gcp` repositories will automatically redeployed to the `kind` cluster.

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

#### Deploying a workload cluster

After your kind management cluster is up and running with Tilt, you can [configure workload cluster settings](#customizing-the-cluster-deployment) and deploy a workload cluster with the following:

```shell
$ make create-workload-cluster
```

To delete the cluster:

```shell
$ make delete-workload-cluster
```

### Submitting PRs and testing

Pull requests and issues are highly encouraged!
If you're interested in submitting PRs to the project, please be sure to run some initial checks prior to submission:

```shell
$ make lint # Runs a suite of quick scripts to check code structure
$ make test # Runs tests on the Go code
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
[kustomizelinux]: https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md
[timeout]: http://man7.org/linux/man-pages/man1/timeout.1.html
