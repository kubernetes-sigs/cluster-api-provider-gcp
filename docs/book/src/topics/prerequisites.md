# Prerequisites

## Requirements

- Linux or MacOS (Windows isn't supported at the moment).
- A [Google Cloud](https://console.cloud.google.com) account.
- [Packer](https://www.packer.io/intro/getting-started/install.html) and [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html) to build images
- Make to use `Makefile` targets
- Install `coreutils` (for timeout) on *OSX*

### Setup environment variables

```bash
export GCP_REGION="<GCP_REGION>"
export GCP_PROJECT="<GCP_PROJECT>"
# Make sure to use same kubernetes version here as building the GCE image
export KUBERNETES_VERSION=1.22.3
export GCP_CONTROL_PLANE_MACHINE_TYPE=n1-standard-2
export GCP_NODE_MACHINE_TYPE=n1-standard-2
export GCP_NETWORK_NAME=<GCP_NETWORK_NAME or default>
export CLUSTER_NAME="<CLUSTER_NAME>"
```

### Setup a Network and Cloud NAT

Google Cloud accounts come with a `default` network which can be found under
[VPC Networks](https://console.cloud.google.com/networking/networks).
If you prefer to create a new Network, follow [these instructions](https://cloud.google.com/vpc/docs/using-vpc#create-auto-network).

#### Cloud NAT
This infrastructure provider sets up Kubernetes clusters using a
[Global Load Balancer](https://cloud.google.com/load-balancing/) with a public ip address.

Kubernetes nodes, to communicate with the control plane, pull container images from registered (e.g. gcr.io or dockerhub) need to have NAT access or a public ip.
By default, the provider creates Machines without a public IP.

To make sure your cluster can communicate with the outside world, and the load balancer, you can create a [Cloud NAT](https://cloud.google.com/nat/docs/overview) in the region you'd like your Kubernetes cluster to live in by following [these instructions](https://cloud.google.com/nat/docs/using-nat#create_nat).

> NB: The following commands needs to be run if `${GCP_NETWORK_NAME}` is set to `default`

```bash
# Ensure if network list contains default network
gcloud compute networks list --project="${GCP_PROJECT}"

gcloud compute networks describe "${GCP_NETWORK_NAME}" --project="${GCP_PROJECT}"

# Ensure if firewall rules are enabled
$ gcloud compute firewall-rules list --project "$GCP_PROJECT"

# Create routers
gcloud compute routers create "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" --region="${GCP_REGION}" --network="default"

# Create NAT
gcloud compute routers nats create "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" --router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter"
--nat-all-subnet-ip-ranges --auto-allocate-nat-external-ips
```

### Create a Service Account

To create and manage clusters, this infrastructure provider uses a service account to authenticate with GCP's APIs.

From your cloud console, follow [these instructions](https://cloud.google.com/iam/docs/creating-managing-service-accounts#creating) to create a new service account with `Editor` permissions.

If you plan yo use GKE the the service account will also need the `iam.serviceAccountTokenCreator` role.

Afterwards, generate a JSON Key and store it somewhere safe.

### Building images

> NB: The following commands should not be run as `root` user.

```bash
# Export the GCP project id you want to build images in.
export GCP_PROJECT_ID=<project-id>

# Export the path to the service account credentials created in the step above.
export GOOGLE_APPLICATION_CREDENTIALS=</path/to/serviceaccount-key.json>

# Clone the image builder repository if you haven't already.
git clone https://github.com/kubernetes-sigs/image-builder.git image-builder

# Change directory to images/capi within the image builder repository
cd image-builder/images/capi

# Run the Make target to generate GCE images.
make build-gce-ubuntu-2004

# Check that you can see the published images.
gcloud compute images list --project ${GCP_PROJECT_ID} --no-standard-images --filter="family:capi-ubuntu-2004-k8s"

# Export the IMAGE_ID from the above
export IMAGE_ID="projects/${GCP_PROJECT_ID}/global/images/<image-name>"
```


### Clean-up

Delete the NAT gateway
```bash
gcloud compute routers nats delete "${CLUSTER_NAME}-mynat" --project="${GCP_PROJECT}" \
--router-region="${GCP_REGION}" --router="${CLUSTER_NAME}-myrouter" --quiet || true
```

Delete the router
```bash
gcloud compute routers delete "${CLUSTER_NAME}-myrouter" --project="${GCP_PROJECT}" \
--region="${GCP_REGION}" --quiet || true
```

