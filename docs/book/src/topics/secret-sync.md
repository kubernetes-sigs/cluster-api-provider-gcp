# Secret Sync

Configure GKE's [Secret Manager sync feature](https://cloud.google.com/secret-manager/docs/sync-k8-secrets), which synchronizes secrets from Google Secret Manager into native Kubernetes Secrets, using the `secretSyncConfig` field on `GCPManagedControlPlane`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: mygcpmanagedcontrolplane
spec:
  secretSyncConfig:
    enabled: true
```

This is a distinct feature from the [Secret Manager add-on](./secret-manager.md) (`secretManagerConfig`), which mounts secrets as volumes via a CSI driver instead of creating Kubernetes Secrets. The two features can be enabled independently or together.

Once enabled on the cluster, synchronization is configured per-workload using the `SecretProviderClass` and `SecretSync` custom resources.

## Automatic re-sync

By default, synced secrets are only refreshed when new versions are explicitly synced. To periodically check Secret Manager for new secret versions and re-sync them, enable rotation and optionally set the interval (GKE defaults to 2 minutes if unset):

```yaml
spec:
  secretSyncConfig:
    enabled: true
    rotationConfig:
      enabled: true
      rotationInterval: 5m
```

## IAM requirements

The synchronization controller needs the `roles/secretmanager.secretAccessor` role on the secrets it syncs, granted to the Kubernetes service account's associated IAM identity (typically via [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)):

```shell
gcloud secrets add-iam-policy-binding SECRET_NAME \
  --member="principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/PROJECT_ID.svc.id.goog/subject/ns/NAMESPACE/sa/KSA_NAME" \
  --role="roles/secretmanager.secretAccessor"
```

Omitting `secretSyncConfig` leaves the feature disabled. The field is mutable and can be changed on an existing cluster.
