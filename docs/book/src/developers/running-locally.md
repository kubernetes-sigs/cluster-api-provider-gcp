# Running CAPG Locally

### Using Tilt

Both of the [Tilt](https://tilt.dev) setups below will get you started developing CAPG in a local KinD cluster. The main difference is the number of components you will build from source and the scope of the changes you'd like to make. If you only want to make changes in CAPG, then follow [CAPG instructions](./setup-tutorial.md#tilt-for-dev-in-capg). This will save you from having to build all of the images for CAPI, which can take a while. If the scope of your development will span both CAPG and CAPI, then follow the [CAPI and CAPG instructions](#tilt-for-dev-in-both-capg-and-capi).

**Note:** On macOS port 5000 (the default port for the KinD registry) is often taken by AirPlay Receiver, so you might
get an error message about that port being used.

If that happens, re-run `make tilt-up` with `KIND_REGISTRY_PORT=<PORT_NUMBER>` and KinD will use the specified port
number instead. Make sure to choose a port that is not being used by another process!

If a previous attempt left behind a `kind-registry` container or a `capg` KinD cluster, you can run
`docker rm -f kind-registry` and `make kind-reset` to get back to a blank slate before trying again (`make kind-reset`
also deletes `capg-e2e` and `clusterapi` clusters).

Alternatively, you can also run:

```shell
./scripts/setup-dev-enviroment.sh
```

It will setup the network, if you already setup the network you can skip this step for that just run:

```shell
./scripts/setup-dev-enviroment.sh --skip-init-network
```

By default, the Cluster API components deployed by Tilt have experimental features turned off.
If you would like to enable these features, add `extra_args` as specified in [The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/tilt.html#create-a-tilt-settingsjson-file).

Once your KinD management cluster is up and running, you can [deploy a workload cluster](#deploying-a-workload-cluster).

To tear down the KinD cluster built by the command above, just run:

```shell
make kind-reset
```

And if you need to cleanup the network setup you can run:

```shell
./scripts/setup-dev-enviroment.sh --clean-network
```

#### Tilt for dev in both CAPG and CAPI

If you want to develop in both CAPI and CAPG at the same time, then this is the path for you.

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the cluster-api repo. The instructions will walk you through cloning the Cluster API (CAPI) repository and configuring Tilt to use KinD to deploy the cluster api management components.

> you may wish to checkout out the correct version of CAPI to match the [version used in CAPG][go.mod]

Note that `tilt up` will be run from the `cluster-api repository` directory and the `tilt-settings.json` file will point back to the `cluster-api-provider-gcp` repository directory. Any changes you make to the source code in `cluster-api` or `cluster-api-provider-gcp` repositories will automatically redeployed to the KinD cluster.

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

After your KinD management cluster is up and running with Tilt, ensure you have all the environment variables set as
described in [Tilt for dev in CAPG](./setup-tutorial.md#tilt-for-dev-in-capg), and deploy a workload cluster with the following:

```shell
make create-workload-cluster
```

To delete the cluster:

```shell
make delete-workload-cluster
```

[go.mod]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/master/go.mod
