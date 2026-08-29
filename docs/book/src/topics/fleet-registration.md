# Fleet Registration

[GKE Fleets](https://cloud.google.com/kubernetes-engine/fleet-management/docs/fleet-creation) let you group, view, and manage GKE clusters across projects as a single logical unit. Registering a cluster into a fleet creates a *Membership* resource for it in the fleet's host project.

CAPG can register a `GCPManagedControlPlane`'s underlying GKE cluster into a fleet as a Membership automatically, instead of relying on an external process (for example a cron job running `gcloud container fleet memberships register`).

## How do I register a cluster into a fleet?

Add a `fleet` field to `GCPManagedControlPlane.spec`. Setting the field at all — even to an empty object — enables registration; omitting it leaves the cluster unregistered:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: capg-gke-control-plane
spec:
  project: my-project
  location: us-central1
  fleet: {}
```

By default the Membership is created in the cluster's own project (`spec.project`), matching the most common single-project setup.

## Registering into a separate fleet-host project

Some organizations host their fleet in a dedicated project rather than the cluster's own project. Set `fleet.project` to override the default:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedControlPlane
metadata:
  name: capg-gke-control-plane
spec:
  project: my-project
  location: us-central1
  fleet:
    project: my-fleet-host-project
```

## IAM requirements

The [Quick Start](../quick-start.md) guide has you grant the provider's service account project-level `Editor` in the cluster's own project. That role should already include the `gkehub.*` permissions needed to manage Memberships in that same project, but this is worth verifying for your organization's IAM policies rather than assuming it — for example with:

```bash
gcloud iam roles describe roles/editor --format="value(includedPermissions)" | grep gkehub
```

If you register into a **separate fleet-host project** (`fleet.project` set to a project other than the cluster's own), the `Editor` grant in the cluster's project does not help — IAM roles are scoped per-project. The service account needs an **additional** role bound in the fleet-host project specifically, for example `roles/gkehub.admin`, or a narrower custom role covering Membership create/get/delete.

## Unsetting or changing fleet registration

CAPG reconciles fleet membership like any other field: it tracks which project a Membership was actually created in (in `status.fleetMembership`), independent of what the current spec says. As a result:

- Unsetting `spec.fleet` (while the cluster keeps running) deregisters the Membership — it does not just stop reconciling it and leave it behind.
- Changing `fleet.project` deregisters the Membership in the old project and registers a new one in the new project.
- Deleting the `GCPManagedControlPlane` deregisters the Membership as part of teardown.

You can confirm current registration state via the `GKEControlPlaneFleetRegistered` condition and `status.fleetMembership.project` on the `GCPManagedControlPlane`.
