# Enabling GKE Support

Enabling GKE support is done via the **GKE** feature flag by setting it to true. This can be done before running `clusterctl init` by using the **EXP_CAPG_GKE** environment variable:

```shell
export EXP_CAPG_GKE=true
clusterctl init --infrastructure gcp
```

> IMPORTANT: To use GKE the service account used for CAPG will need the `iam.serviceAccountTokenCreator` role assigned.
