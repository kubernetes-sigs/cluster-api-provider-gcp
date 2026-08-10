# Secret Manager

Configure GKE's [Secret Manager add-on](https://cloud.google.com/kubernetes-engine/docs/how-to/secret-manager-csi-driver), which lets workloads mount secrets from Google Secret Manager via a CSI driver, using the `secretManagerConfig` field on `GCPManagedControlPlane`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: mygcpmanagedcontrolplane
spec:
  secretManagerConfig:
    enabled: true
```

## Automatic secret rotation

By default, secrets mounted via the CSI driver are only refreshed when the mounting pod restarts. To periodically refresh cached secrets, enable rotation and optionally set the interval (GKE defaults to 2 minutes if unset):

```yaml
spec:
  secretManagerConfig:
    enabled: true
    rotationConfig:
      enabled: true
      rotationInterval: 5m
```

## IAM requirements

Workloads reading secrets need the `roles/secretmanager.secretAccessor` role on the secrets they mount, granted to the Kubernetes service account's associated IAM identity (typically via [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)):

```shell
gcloud secrets add-iam-policy-binding SECRET_NAME \
  --member="principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/PROJECT_ID.svc.id.goog/subject/ns/NAMESPACE/sa/KSA_NAME" \
  --role="roles/secretmanager.secretAccessor"
```

Omitting `secretManagerConfig` leaves the add-on disabled. The field is mutable and can be changed on an existing cluster.
