# ClusterClass

- **Feature status:** Experimental
- **Feature gate:** `ClusterTopology=true`

[ClusterClass](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/index.html) is a collection of templates that define a topology (control plane and machine deployments) to be used to continuously reconcile one or more Clusters. It is built on top of the existing Cluster API resources and provides a set of tools and operations to streamline cluster lifecycle management while maintaining the same underlying API.

<aside class="note warning">

<h1>Warning</h1>

ClusterClass is an experimental core CAPI feature and, as such, it may behave unreliably until promoted to the main repository. It is expected to be graduated with the release of [CAPI v1.9](https://github.com/kubernetes-sigs/cluster-api/milestone/38).

</aside>

CAPG supports the creation of clusters via Cluster Topology for **self-managed clusters only**.
