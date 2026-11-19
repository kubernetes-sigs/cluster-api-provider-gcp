# Running CAPG Locally

When developing CAPG, running it locally is a really great way to try out your changes before it hits CI. It can also
be a great way of developing CAPG alongside Cluster API.

## Starting & Stopping

<aside class="note">

By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/core/tilt.html#create-a-tilt-settings-file). Only `core`,
`kubeadm-bootstrap` and `gcp` are read here.
</aside>

To run locally you need to have a working [Tilt Setup](./setup-tutorial.md), and then to start your local environment run:

```shell
make tilt-up
```

To tear down the KinD cluster, just run:

```shell
make kind-reset
```

## Debugging

If you would like to debug CAPG you can run the provider with [delve](https://github.com/go-delve/delve), a Go debugger tool. This will then allow you
to attach to delve and troubleshoot the processes.

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

If you use these settings when running Tilt you can connect to `delve` and debug the problem at hand

For vscode, you can use a launch configuration like this:

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

Create a new configuration and add it to the "Debug" menu. To configure debugging in GoLand/IntelliJ follow [these instructions](https://www.jetbrains.com/help/go/attach-to-running-go-processes-with-debugger.html#step-3-create-the-remote-run-debug-configuration-on-the-client-computer).

Alternatively, you may use delve straight from the CLI by executing a command like this:

```shell
delve -a tcp://localhost:30000
```

Note that installing latest can cause mismatches with the version that runs inside the pod. To make sure it's the same
check the version in the [Tiltfile](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/Tiltfile).

If something on your machine is using port 30000, then change `debug.gcp.port` in your tilt-settings.json and connect
to that port instead.

## Deploying a workload cluster

When running locally it's possible to get CAPG to deploy whole workload clusters so changes can be tested end-to-end.
To create a workload cluster, you first need to decide if you want to create a Self-Managed Cluster, or a Managed (GKE)
Cluster.

### Self-Managed Clusters

In order to deploy a self-managed workload cluster you will need to pull a node image (this is not the same as the
Docker Image that runs CAPG but is a VM image that bootstraps your Kubernetes nodes).

To do this, first look in our
[CI configuration](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/test/e2e/config/gcp-ci.yaml)
for the current tested Ubuntu version. Then list available images with the following `gcloud` command (note you'll need
to be logged into an account to do this, but you don't need to be given special access to this project):

```shell
gcloud compute images list --project k8s-staging-cluster-api-gcp --no-standard-images \
  --filter="name~cluster-api-ubuntu-<UBUNTU_VERSION>-.*-nightly" --format="value(name)"
```

This will list all available versions of the image, pick the one that corresponds to the Kubernetes version you want to
run.

<aside class="note">

If you need to, it's possible to build these images yourself, to do this you can follow the instructions [here](https://image-builder.sigs.k8s.io/capi/providers/gcp.html)
</aside>

#### Self-Managed Cluster Environment Variables

Run the following script to set up the environment correctly, setting each value as you go:

```shell
export CLUSTER_NAME="<CLUSTER_NAME>"
export CONTROL_PLANE_MACHINE_COUNT=1
export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default>
export GCP_NODE_MACHINE_TYPE=n1-standard-2
export GCP_PROJECT="<GCP_PROJECT>"
export GCP_REGION="<GCP_REGION>"
export IMAGE_ID=projects/k8s-staging-cluster-api-gcp/global/images/<IMAGE_NAME>
export KUBERNETES_VERSION="<KUBERNETES_VERSION>"
export WORKER_MACHINE_COUNT=1
```

`IMAGE_NAME` should be taken from the previous step and `KUBERNETES_VERSION` needs to match the one in the image name.
Specifically the `IMAGE_NAME` will be of the format `...-vX-Y-Z-nightly`, therefore the Kubernetes version should be
`vX.Y.Z`.

In addition, note that if `GCP_NETWORK_NAME` does not exist, CAPG will create it along with other networking resources.
These resources are cleared up when the cluster is deleted. If the value `default` is used then CAPG creates no
network or Cloud NAT, and you'll need to follow [these steps](../prerequisites.md#configure-network-and-cloud-nat) first
to ensure the nodes can pull images correctly.

#### Deploying A Self-Managed Cluster

You can then deploy a workload cluster with the following:

```shell
make create-workload-cluster
```

This will apply the self-managed template, wait for a KubeConfig to be available and then install Calico as a
[CNI](../self-managed/cni.md). If you'd like to test a different CNI then that will be a manual installation.

To later delete the cluster:

```shell
make delete-workload-cluster
```

### Managed (GKE) Clusters

Deploying a managed cluster is easier as GCP handles a lot of the work for you.

#### Enabling The Feature Gate

As GKE is still an experimental feature, it requires a feature gate to be set when running CAPG. As such, when running
`make tilt-up` you need to append `EXP_CAPG_GKE=true`, so run the following command

```shell
EXP_CAPG_GKE=true make tilt-up
```

If you're already running Tilt because of previous tutorials you'll need to restart it with the command above.

#### Managed (GKE) Cluster Environment Variables

In a different shell (as tilt will take up the first) fill in the placeholders and set the following environment variables

```shell
export CLUSTER_NAME="<CLUSTER_NAME>"
export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default>
export GCP_PROJECT="<GCP_PROJECT>"
export GCP_REGION="<GCP_REGION>"
# This will provision as a regional cluster, so the number of worker nodes need to be a multiple of
# the number of zones in the region (likely 3)
export WORKER_MACHINE_COUNT=3
```

#### Deploying A Managed (GKE) Cluster

From the root of the repository, run the following command

```shell
./hack/tools/bin/envsubst < templates/cluster-template-gke.yaml | ./hack/tools/bin/kubectl --context kind-capg apply -f -
```

To later delete the cluster run

```shell
./hack/tools/bin/kubectl --context kind-capg delete cluster $CLUSTER_NAME
```

## Developing both CAPG and CAPI

If you want to develop in both CAPI and CAPG at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/core/tilt.html) in the cluster-api repo. The
instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use KinD to deploy
the cluster api management components.

You may wish to check out the correct version of CAPI to match the [version used in CAPG][go.mod]

Note that `tilt up` will be run from the `cluster-api` repository directory and the `tilt-settings.json` file will
point back to the `cluster-api-provider-gcp` repository directory. Any changes you make to the source code in
`cluster-api` or `cluster-api-provider-gcp` repositories will automatically re-deploy to the KinD cluster.

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

The cluster-api management components that are deployed are configured at the `/config` folder of each repository
respectively. Making changes to those files will trigger a re-deploy of the management cluster components.

[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/go.mod
