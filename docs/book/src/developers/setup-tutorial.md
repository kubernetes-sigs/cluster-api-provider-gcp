# Setup CAPG Tutorial

### Get the source

```shell
git clone https://github.com/kubernetes-sigs/cluster-api-provider-gcp
cd cluster-api-provider-gcp
```

### Get familiar with basic concepts

This provider is modeled after the upstream Cluster API project. To get familiar
with Cluster API resources, concepts and conventions ([such as CAPI and CAPG](https://cluster-api.sigs.k8s.io/reference/glossary.html#c)), refer to the [Cluster API Book](https://cluster-api.sigs.k8s.io/).

## CAPG Node images

In order to deploy a workload cluster you will need to build the node images to use, for that you can reference the [image-builder](https://github.com/kubernetes-sigs/image-builder)
project, also you can read the [image-builder book](https://image-builder.sigs.k8s.io/)

Please refer to the image-builder documentation in order to get the latest requirements to build the node images.

To build the node images for GCP: [https://image-builder.sigs.k8s.io/capi/providers/gcp.html](https://image-builder.sigs.k8s.io/capi/providers/gcp.html)

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

To build a KinD cluster and start Tilt, just run:

```shell
make tilt-up
```
