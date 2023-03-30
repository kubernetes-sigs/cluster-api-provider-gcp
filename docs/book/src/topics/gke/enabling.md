# Enabling GKE Support

Enabling GKE support is done via the **GKE** and **Machine Pool** feature flags by setting them to true. This can be done before running `clusterctl init` by using the **EXP_CAPG_GKE** and **EXP_MACHINE_POOL** environment variables:

```shell
export EXP_CAPG_GKE=true
export EXP_MACHINE_POOL=true
clusterctl init --infrastructure gcp
```

> IMPORTANT: To use GKE the service account used for CAPG will need the `iam.serviceAccountTokenCreator` role assigned.
