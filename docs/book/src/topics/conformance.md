# Running Conformance tests

## Required environment variables
- Set the GCP region

```console
export GCP_REGION=us-east4
```

- Set the GCP project to use

```console
export GCP_PROJECT=your-project-id
```

- Set the path to the service account

```console
export GOOGLE_APPLICATION_CREDENTIALS=path/to/your/service-account.json
```

## Optional environment variables

- Set a specific name for your cluster

```console
export CLUSTER_NAME=test1
```

- Set a specific name for your network

```console
export NETWORK_NAME=test1-mynetwork
```

- Skip cleaning up the project resources

```console
export SKIP_CLEANUP=1
```

## Running the conformance tests

```console
scripts/ci-conformance.sh
```
