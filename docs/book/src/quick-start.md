# Getting started with CAPG

In this section we'll cover the basics of how to prepare your environment to use Cluster API Provider for GCP.

<aside class="note">

<h1>Tip</h1>

This covers the specifics of CAPG but won't go into detail of core CAPI basics. For more information on how CAPI works and how to interact with different providers, you can refer to [CAPI Quick Start](https://cluster-api.sigs.k8s.io/user/quick-start).

</aside>

Before installing CAPG, your Kubernetes cluster has to be transformed into a CAPI management cluster. If you have already done this, you can jump directly to the next section: [Installing CAPG](#installing-capg). If, on the other hand, you have an existing Kubernetes cluster that is not yet configured as a CAPI management cluster, you can follow the guide from the [CAPI book](https://cluster-api.sigs.k8s.io/user/quick-start#initialize-the-management-cluster).

## Requirements

- Linux or MacOS (Windows isn't supported at the moment).
- A [Google Cloud](https://console.cloud.google.com) account.
- [Packer](https://www.packer.io/intro/getting-started/install.html) and [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html) to build images
- `make` to use `Makefile` targets
- Install `coreutils` (for timeout) on *OSX*

### Credentials

To create and manage clusters, CAPG uses a GCP service account to authenticate with GCP's APIs. There are two supported authentication methods.

First, [create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts#creating) with `Editor` permissions. If you plan to use GKE the service account will also need the `iam.serviceAccountTokenCreator` role.

#### Service Account JSON Key

Generate a JSON Key for the service account and store it somewhere safe. This key will be base64-encoded and provided to CAPG at installation time (see [Installing CAPG](#installing-capg)).

#### Workload Identity Federation (GKE management clusters)

If your CAPI management cluster runs on GKE, [Workload Identity Federation](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) is the preferred authentication method. It eliminates the need to manage JSON key files by binding the CAPG Kubernetes ServiceAccount to a GCP service account.

Enable Workload Identity on your GKE management cluster (if not already enabled):
```
gcloud container clusters update <MANAGEMENT_CLUSTER> \
  --workload-pool=<PROJECT_ID>.svc.id.goog \
  --region <REGION>
```

Grant the Workload Identity User role so the CAPG Kubernetes ServiceAccount can impersonate the GCP service account:
```
export GCP_SA_EMAIL=<gcp-service-account>@<PROJECT_ID>.iam.gserviceaccount.com

gcloud iam service-accounts add-iam-policy-binding "${GCP_SA_EMAIL}" \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:<PROJECT_ID>.svc.id.goog[capg-system/capg-manager]"
```

Then deploy CAPG using the `config/wif` overlay, substituting your GCP service account email:
```
export GCP_SA_EMAIL=<gcp-service-account>@<PROJECT_ID>.iam.gserviceaccount.com

kustomize build config/wif/ | envsubst | kubectl apply -f -
```

CAPG will authenticate via the GKE metadata server automatically — no credentials secret is needed.


### Installing CAPG

There are two major provider installation paths: using `clusterctl` or the `Cluster API Operator`.

`clusterctl` is a command line tool that provides a simple way of interacting with CAPI and is usually the preferred alternative for those who are getting started. It automates fetching the YAML files defining provider components and installing them.

The `Cluster API Operator` is a Kubernetes Operator built on top of `clusterctl` and designed to empower cluster administrators to handle the lifecycle of Cluster API providers within a management cluster using a declarative approach. It aims to improve user experience in deploying and managing Cluster API, making it easier to handle day-to-day tasks and automate workflows with GitOps. Visit the CAPI Operator quickstart if you want to experiment with this tool.

You can opt for the tool that works best for you or explore both and decide which is best suited for your use case.

#### clusterctl

The Service Account you created will be used to interact with GCP and it must be base64 encoded and stored in a environment variable before installing the provider via `clusterctl`.

```
export GCP_B64ENCODED_CREDENTIALS=$( cat /path/to/gcp-credentials.json | base64 | tr -d '\n' )
```
Finally, let's initialize the provider.
```
clusterctl init --infrastructure gcp
```
This process may take some time and, once the provider is running, you'll be able to see the `capg-controller-manager` pod in your CAPI management cluster.

#### Cluster API Operator

You can refer to the Cluster API Operator book [here](https://cluster-api-operator.sigs.k8s.io/01_user/02_quick-start) to learn about the basics of the project and how to install the operator.

When using Cluster API Operator, secrets are used to store credentials for cloud providers and not environment variables, which means you'll have to create a new secret containing the base64 encoded version of your GCP credentials and it will be referenced in the yaml file used to initialize the provider. As you can see, by using Cluster API Operator, we're able to manage provider installation declaratively.

Create GCP credentials secret.
```
export CREDENTIALS_SECRET_NAME="gcp-credentials"
export CREDENTIALS_SECRET_NAMESPACE="default"
export GCP_B64ENCODED_CREDENTIALS=$( cat /path/to/gcp-credentials.json | base64 | tr -d '\n' )

kubectl create secret generic "${CREDENTIALS_SECRET_NAME}" --from-literal=GCP_B64ENCODED_CREDENTIALS="${GCP_B64ENCODED_CREDENTIALS}" --namespace "${CREDENTIALS_SECRET_NAMESPACE}"
```
Define CAPG provider declaratively in a file `capg.yaml`.
```
apiVersion: v1
kind: Namespace
metadata:
  name: capg-system
---
apiVersion: operator.cluster.x-k8s.io/v1alpha2
kind: InfrastructureProvider
metadata:
 name: gcp
 namespace: capg-system
spec:
 version: v1.8.0
 configSecret:
   name: gcp-credentials
```
After applying this file, Cluster API Operator will take care of installing CAPG using the set of credentials stored in the specified secret.
```
kubectl apply -f capg.yaml
```
