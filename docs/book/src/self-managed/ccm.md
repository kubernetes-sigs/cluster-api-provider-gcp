# Cloud Controller Manager

This section details the integration with the [`cloud-provider-gcp`](https://github.com/kubernetes/cloud-provider-gcp)
for GCP.

## History

At the inception of Kubernetes many cloud providers were encouraged to bundle functionality that interacted with their
systems. This functionality would create cloud provider specific resources via API calls rather than having to build
custom functionality in the cloud provider to host Kubernetes. These integrations were known as the "in-tree" cloud
providers.

However, in Kubernetes 1.31 these "in-tree" providers were removed (see this
[blog post](https://www.cncf.io/blog/2024/09/27/why-kubernetes-is-removing-in-tree-cloud-provider-integration-support-in-v1-31-and-how-it-can-affect-you/)
for context), in favour of separately deployed controllers that were maintained by the cloud providers themselves.
By doing it this way Kubernetes core could move at the speed that was desired while the cloud providers could innovate
independently of that.

## How does this affect CAPG

This is particularly relevant for CAPG, especially when working with self-managed clusters because when we are
provisioning a cluster, we rely on the GCP Cloud Controller Manager (CCM). As such our templates (which can
be applied through `clusterctl`) needed to change to account for this.

The changes themselves are not particularly important however, this does mean that *versions of Kubernetes before 1.31
are untested with CAPG's current templates*. This does not mean they will not work, it's simply that we don't
test these versions. Furthermore, as per CAPI's
[support matrix](https://cluster-api.sigs.k8s.io/reference/versions.html#kubernetes-versions-support) only versions
1.31 and beyond are supported in any event.

## What do the templates do?

Now that we have to provide the CCM in the cluster we make use of Cluster API's [`ClusterResourceSet`](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20200220-cluster-resource-set.md).
This ensures that every cluster you create adds a correctly configured CCM such that provisioning a cluster will work.

We also make use of `cloud-provider: external` on the Kubelet and the `controller-manager` as well as
`cloud.config` with `multizone = true`.

## What CCM version should I use?

Choosing a CCM version is relatively easy, however it has to be aligned to the version of Kubernetes that you want to
use. As such the templates we provide contain a default set of versions. However, we recommend that you pin to a specific patch version and regularly bump as new patches appear and
to match the version of Kubernetes you're using.

The version numbers for CCM are structured as `v<K8SMINOR>.<K8SPATCH>.<CCMPATCH>`. Therefore, for Kubernetes 1.36.4 you
would need CCM version `v36.4.x` where x is one of the published patch versions of CCM. CCM releases can be found at
`https://registry.k8s.io/v2/cloud-provider-gcp/cloud-controller-manager/tags/list` and a useful shell snippet for
listing them is:

```shell
curl -fsSL https://registry.k8s.io/v2/cloud-provider-gcp/cloud-controller-manager/tags/list | jq -r '.tags[]'
```

The Kubernetes [version skew policy](https://kubernetes.io/releases/version-skew-policy/) states that the version of
CCM can be at maximum one minor older than the API server, which is why we recommend pinning to the same major version
of CCM as the Kubernetes minor. In addition to that the CCM must not be newer than the API Server. It's also worth 
noting that versions `v31.x` do not exist, if that is your desired version use `v30.x` instead.

### Setting the Version (and other variables)

To set the version of CCM and the CIDR block for the pods you can use the following snippet

```shell
export CCM_VERSION=...
export POD_CIDR=...
```

After this if you run `clusterctl generate cluster` these variables will be taken into account.

## Troubleshooting

The key symptom of a problem with this approach is nodes that never become ready, stuck with a
`node.cloudprovider.kubernetes.io/uninitialized` taint. To see if this taint is attached to your nodes you can run the
following `kubectl` command:

```shell
kubectl get nodes -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.taints[*].key}{"\n"}{end}'
```

## Notes

- Though our manifests add the RBAC for `LoadBalancer` services our tests don't exercise them so you should consider
  that feature "unsupported"
- The topology flavour requires `CNI_RESOURCES` to function correctly



