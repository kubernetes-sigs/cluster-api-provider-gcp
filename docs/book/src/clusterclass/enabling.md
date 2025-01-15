# Enabling ClusterClass Support

Enabling ClusterClass support is done via the **ClusterTopology** feature flag by setting it to true. This can be done before running `clusterctl init` by using the **CLUSTER_TOPOLOGY** environment variable:

```shell
export CLUSTER_TOPOLOGY=true
clusterctl init --infrastructure gcp
```
