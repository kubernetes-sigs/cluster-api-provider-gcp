# Prerequisites

## Requirements

- Linux or MacOS (Windows isn't supported at the moment).
- A [Google Cloud](https://console.cloud.google.com) account.
- [Packer](https://www.packer.io/intro/getting-started/install.html) and [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html) to build images
- Make to use `Makefile` targets

### Setup a Network and Cloud NAT

Google Cloud accounts come with a `default` network which can be found under
[VPC Networks](https://console.cloud.google.com/networking/networks).
If you prefer to create a new Network, follow [these instructions](https://cloud.google.com/vpc/docs/using-vpc#create-auto-network).

#### Cloud NAT
This infrastructure provider sets up Kubernetes clusters using a
[Global Load Balancer](https://cloud.google.com/load-balancing/) with a public ip address.

Kubernetes nodes, to communicate with the control plane, pull container images from registried (e.g. gcr.io or dockerhub) need to have NAT access or a public ip.
By default, the provider creates Machines without a public IP.

To make sure your cluster can communicate with the outside world, and the load balancer, you can create a [Cloud NAT](https://cloud.google.com/nat/docs/overview) in the region you'd like your Kubernetes cluster to live in by following [these instructions](https://cloud.google.com/nat/docs/using-nat#create_nat).

### Create a Service Account

To create and manager clusters, this infrastructure providers uses a service account to authenticate with GCP's APIs.

From your cloud console, follow [these instructions](https://cloud.google.com/iam/docs/creating-managing-service-accounts#creating) to create a new service account with `Editor` permissions. Afterwards, generate a JSON Key and store it somewhere safe.

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
make build-gce-default

# Check that you can see the published images.
gcloud compute images list --project ${GCP_PROJECT_ID} --no-standard-images --filter="family:capi-ubuntu-1804-k8s"
```

