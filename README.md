# Kubernetes cluster-api-provider-gcp Project

This repository hosts a concrete implementation of a provider for [Google Cloud Platform](https://cloud.google.com/) for the [cluster-api project](https://github.com/dims/cluster-api).

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

* Join our Cluster API working group sessions
  * Weekly on Wednesdays @ 10:00 PT on [Zoom](https://zoom.us/j/166836624)
  * Previous meetings: \[ [notes](https://docs.google.com/document/d/16ils69KImmE94RlmzjWDrkmFZysgB2J4lGnYMRN89WM/edit) | [recordings](https://www.youtube.com/playlist?list=PL69nYSiGNLP29D0nYgAGWt1ZFqS9Z7lw4) \]

You can reach the maintainers of this project at:

- [Slack](http://slack.k8s.io/): #cluster-api
- [Mailing List](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Getting Started

### Prerequisites

1. Install `kubectl` (see [here](http://kubernetes.io/docs/user-guide/prereqs/)).
1. Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/).
1. Install a [driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md) for minikube. For Linux, we recommend kvm2. For MacOS, we recommend VirtualBox.
1. Install `kustomize` (see [here](https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md)).   
Note: Until incompatible changes introduced in kustomize v2.0.0 are addressed by ClusterAPI, you must use a pre-v2 version of kustomize, e.g., [v1.0.11](https://github.com/kubernetes-sigs/kustomize/releases/tag/v1.0.11).
1. Build the `clusterctl` tool

   ```bash
   git clone https://github.com/kubernetes-sigs/cluster-api-provider-gcp $GOPATH/src/sigs.k8s.io/cluster-api-provider-gcp
   cd $GOPATH/src/sigs.k8s.io/cluster-api-provider-gcp
   make clusterctl
   ```

### Cluster Creation

1. Create the `cluster.yaml`, `machines.yaml`, `provider-components.yaml`, and `addons.yaml` files, and create GCP serviceaccounts:

   ```bash
   cd cmd/clusterctl/examples/google
   ./generate-yaml.sh
   cd ../../../..
   kustomize build config/default/ > cmd/clusterctl/examples/google/out/provider-components.yaml
   echo "---" >> cmd/clusterctl/examples/google/out/provider-components.yaml
   kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> cmd/clusterctl/examples/google/out/provider-components.yaml
   ```

1. Create a cluster:

   Set the generated serviceaccount as a local environment variable so that the `clusterctl` process uses the same google
   credentials as do the processes running in minikube and in the final cluster.

   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS=cmd/clusterctl/examples/google/out/machine-controller-serviceaccount.json

   ./bin/clusterctl create cluster --provider google -c cmd/clusterctl/examples/google/out/cluster.yaml -m cmd/clusterctl/examples/google/out/machines.yaml -p cmd/clusterctl/examples/google/out/provider-components.yaml -a cmd/clusterctl/examples/google/out/addons.yaml --bootstrap-type=minikube --bootstrap-flags="kubernetes-version=v1.12.0"
   ```

To choose a specific minikube driver, please use the `--vm-driver` command line parameter. For example to use the kvm2 driver with clusterctl you woud add `--vm-driver kvm2`.

Adding `--bootstrap-type=minikube --bootstrap-flags="kubernetes-version=v1.12.0"` enforces bootstrap cluster to be in a version supporting sub-resources in CRDs, used by this code. Kubernetes before version v1.12 doesn't support them out-of-the-box.

Additional advanced flags can be found via help.

```bash
./bin/clusterctl create cluster --help
```

### Interacting with your cluster

Once you have created a cluster, you can interact with the cluster and machine
resources using kubectl:

```bash
kubectl --kubeconfig=kubeconfig get clusters
kubectl --kubeconfig=kubeconfig get machines
kubectl --kubeconfig=kubeconfig get machines -o yaml
```

### Cluster Deletion

This guide explains how to delete all resources that were created as part of
your GCP Cluster API Kubernetes cluster.

1. Remember the service accounts that were created for your cluster

   ```bash
   export MASTER_SERVICE_ACCOUNT=$(kubectl --kubeconfig=kubeconfig get cluster -o=jsonpath='{.items[0].metadata.annotations.gce\.clusterapi\.k8s\.io\/service-account-k8s-master}')
   export WORKER_SERVICE_ACCOUNT=$(kubectl --kubeconfig=kubeconfig get cluster -o=jsonpath='{.items[0].metadata.annotations.gce\.clusterapi\.k8s\.io\/service-account-k8s-worker}')
   export INGRESS_CONTROLLER_SERVICE_ACCOUNT=$(kubectl --kubeconfig=kubeconfig get cluster -o=jsonpath='{.items[0].metadata.annotations.gce\.clusterapi\.k8s\.io\/service-account-k8s-ingress-controller}')
   export MACHINE_CONTROLLER_SERVICE_ACCOUNT=$(kubectl --kubeconfig=kubeconfig get cluster -o=jsonpath='{.items[0].metadata.annotations.gce\.clusterapi\.k8s\.io\/service-account-k8s-machine-controller}')
   ```

1. Remember the name and zone of the master VM and the name of the cluster

   ```bash
   export CLUSTER_NAME=$(kubectl --kubeconfig=kubeconfig get cluster -o=jsonpath='{.items[0].metadata.name}')
   export MASTER_VM_NAME=$(kubectl --kubeconfig=kubeconfig get machines -l set=master | awk '{print $1}' | tail -n +2)
   export MASTER_VM_ZONE=$(kubectl --kubeconfig=kubeconfig get machines -l set=master -o=jsonpath='{.items[0].metadata.annotations.gcp-zone}')
   ```

1. Delete all of the node Machines in the cluster. Make sure to wait for the
corresponding Nodes to be deleted before moving onto the next step. After this
step, the master node will be the only remaining node.

   ```bash
   kubectl --kubeconfig=kubeconfig delete machines -l set=node
   kubectl --kubeconfig=kubeconfig get nodes
   ```

1. Delete any Kubernetes objects that may have created GCE resources on your
behalf, make sure to run these commands for each namespace that you created:

   ```bash
   # See ingress controller docs for information about resources created for
   # ingress objects: https://github.com/kubernetes/ingress-gce
   kubectl --kubeconfig=kubeconfig delete ingress --all

   # Services can create a GCE load balancer if the type of the service is
   # LoadBalancer. Additionally, both types LoadBalancer and NodePort will
   # create a firewall rule in your project.
   kubectl --kubeconfig=kubeconfig delete svc --all

   # Persistent volume claims can create a GCE disk if the type of the pvc
   # is gcePersistentDisk.
   kubectl --kubeconfig=kubeconfig delete pvc --all
   ```

1. Delete the VM that is running your cluster's control plane

   ```bash
   gcloud compute instances delete --zone=$MASTER_VM_ZONE $MASTER_VM_NAME
   ```

1. Delete the roles and service accounts that were created for your cluster

   ```bash
   ./scripts/delete-service-accounts.sh
   ```

1. Delete the Firewall rules that were created for the cluster

   ```bash
   gcloud compute firewall-rules delete $CLUSTER_NAME-allow-cluster-internal
   gcloud compute firewall-rules delete $CLUSTER_NAME-allow-api-public
   ```
