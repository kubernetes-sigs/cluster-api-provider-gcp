# Setup CAPG Tutorial

This tutorial will guide you through getting a local development loop setup for CAPG that utilizes
[Tilt](https://tilt.dev/). This guide assumes you have already run through the steps in our [Prerequisites Guide](./prerequisites.md).

## Get the source

The first step is to clone the repo so that you have a local copy. Follow the commands below to create the
clone.

```shell
git clone https://github.com/kubernetes-sigs/cluster-api-provider-gcp
cd cluster-api-provider-gcp
```

## Setting Up Tilt

From the root of the CAPG repository, run the following to generate a `tilt-settings.json` file with your GCP
service account credentials:

```shell
cat <<EOF > tilt-settings.json
{
  "kustomize_substitutions": {
      "GCP_B64ENCODED_CREDENTIALS": "$(cat PATH_FOR_GCP_CREDENTIALS_JSON | base64 -w0)"
  }
}
EOF
```

To build a KinD cluster and start Tilt, just run:

```shell
make tilt-up
```

<aside class="note">

On macOS port 5000 (the default port for the KinD registry) is often taken by AirPlay Receiver, so you might
get an error message about that port being used.

If that happens, re-run `make tilt-up` with `KIND_REGISTRY_PORT=<PORT_NUMBER>` and KinD will use the specified port
number instead. Make sure to choose a port that is not being used by another process!

If a previous attempt left behind a `kind-registry` container or a `capg` KinD cluster, you can run
`docker rm -f kind-registry` and `make kind-reset` to get back to a blank slate before trying again (`make kind-reset`
also deletes the `capg-e2e` cluster).
</aside>

## Checking It Works

Running `make tilt-up` will give you an option to open a browser-based UI that can be very useful for monitoring the
different pieces in the Tilt stack.

When all of the elements in this UI go green your local development is ready to use!

## Where Next?

To use CAPG to provision a workload cluster, follow the instructions in our [Running CAPG Locally](./running-locally.md)
guide.
