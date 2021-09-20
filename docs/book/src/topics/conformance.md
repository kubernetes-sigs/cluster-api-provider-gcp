# Running Conformance tests

## Required environment variables
- Set the GCP region
```
export GCP_REGION=us-east4 
```

- Set the GCP project to use
```
export GCP_PROJECT=your-project-id 
```

- Set the path to the service account
```
export GOOGLE_APPLICATION_CREDENTIALS=path/to/your/service-account.json 
```

## Optional environment variables
- Set a specific name for your cluster
```
export CLUSTER_NAME=test1
```
- Set a specific name for your network 
```
export NETWORK_NAME=test1-mynetwork
```
- Skip running tests
```
export SKIP_RUN_TESTS=1
```
- Skip cleaning up the project resources 
```
export SKIP_CLEANUP=1
```

## Running the conformance tests
```
hack/ci/e2e-conformance.sh
```

## How to cleanup if you used SKIP_CLEANUP to start hack/ci/e2e-conformance.sh earlier
```
hack/ci/e2e-conformance.sh --cleanup
```

## Gimme Moar! logs!!!
```
hack/ci/e2e-conformance.sh --verbose
```


