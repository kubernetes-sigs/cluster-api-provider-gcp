# Firewall Rules

Cluster API Provider GCP (CAPG) allows you to configure GCP VPC firewall rules for your clusters through the `GCPCluster` and `GCPManagedCluster` (GKE) resources. This feature provides fine-grained control over network access to your cluster infrastructure.

## Overview

Firewall rules are configured through the `network.firewall` field in the `GCPCluster` or `GCPManagedCluster` spec. The firewall configuration supports:

- **Default rule management**: Control whether the provider creates default firewall rules
- **Custom firewall rules**: Define additional firewall rules to meet your specific security requirements

### Lifecycle Behavior

CAPG creates firewall rules during cluster provisioning and deletes them when the cluster is deleted. On every reconcile loop it also compares each rule in the spec against the rule that exists in GCP and **updates** the rule in place when they differ, so changes to the spec are rolled out automatically. Because the rule in GCP is replaced with the spec, any out-of-band change made directly in GCP to a rule that CAPG manages is reverted on the next reconcile.

CAPG matches rules **by name**, and every rule it reconciles is recorded in `status.network.firewallRules`. A recorded rule that the spec no longer asks for is **deleted** on the next reconcile, so removing a rule from the spec removes it from GCP, and renaming one deletes the rule that carried the old name. A rule whose name does not match any rule in the spec is never recorded and never touched, so you are free to manage your own rules in the same network by hand.

A rule that omits `name` is named after its contents, and that name is written back into the spec by the defaulting webhook. Editing the rule afterwards therefore changes its contents but not its name, and it is updated in place like any other rule. Clusters owned by a ClusterClass topology are the exception: the webhook leaves their spec untouched, because the topology controller strips anything it writes on its next apply, so the name is derived again on every reconcile. For those clusters, **editing the contents of an unnamed rule changes its name**, and CAPG creates the rule under the new name and deletes the one under the old. Give the rule an explicit `name` if you want it updated in place instead.

> **Warning:** Because rules are matched by name, a rule that already exists in GCP is **adopted** as soon as a rule in the spec resolves to the same name — CAPG does not check who created it. Remember that CAPG prefixes rule names with the cluster name, so a spec rule named `web` matches an existing `<cluster-name>-web`. Once adopted, the rule is recorded in the status, overwritten to match the spec, and deleted when it leaves the spec or the cluster is deleted. Do not point the spec at a pre-existing rule you intend to keep managing yourself.

Because the spec is the desired state, a rule that is deleted directly in GCP is recreated on the next reconcile. To remove a rule for good, remove it from the spec.

The network a rule belongs to cannot be changed in place; it is not compared and is left untouched.

### Upgrading from CAPG v1.13 and Earlier

Rules that omitted `name` used to be named `<cluster-name>-ingress` or `<cluster-name>-egress`, one per direction no matter how many rules the spec held. They are now named after their contents, so the first reconcile after the upgrade creates each rule under its own name and deletes the rule left behind under the old one.

### Immutable Fields

[GCP cannot modify](https://cloud.google.com/firewall/docs/using-firewalls#updating_firewall_rules) the name, the network, the direction of traffic or the action on match (`allowed` versus `denied`) of an existing firewall rule. Since CAPG updates a rule in place as long as it keeps its name, changing `direction` or switching between `allowed` and `denied` on a named rule is rejected by the webhook: the reconciler would otherwise retry an update GCP always refuses. To make one of those changes, **rename the rule**. The rule under the old name is deleted and the new one is created in its place.

Rules that omit `name` are exempt, because their generated name is derived from their contents: changing the direction of an unnamed rule already changes its name, which replaces the rule instead of updating it.

## Default Firewall Rules

By default, the provider creates two firewall rules to enable cluster functionality:

1. **Health Check Rule** (`allow-<cluster-name>-healthchecks`):
   - Allows TCP ingress traffic on port 6443 from GCP health check IP ranges
   - Source ranges: `35.191.0.0/16`, `130.211.0.0/22`
   - Target: control-plane nodes

2. **Cluster Internal Rule** (`allow-<cluster-name>-cluster`):
   - Allows all ingress traffic between cluster nodes
   - Source/Target: control-plane and worker nodes

### Managing Default Rules

You can control the creation of default firewall rules using the `defaultRulesManagement` field:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      defaultRulesManagement: "Managed"  # or "Unmanaged"
```

**Values:**
- `Managed` (default): CAPG creates and manages default firewall rules
- `Unmanaged`: CAPG does not create or manage default firewall rules

**Important Notes:**
- CAPG allows switching from `Managed` to `Unmanaged` and vice versa. Switching from `Unmanaged` to `Managed` makes CAPG create the default rules if they don't already exist. Switching from `Managed` to `Unmanaged` **deletes** the default rules CAPG already created: they are recorded in `status.network.firewallRules`, and every recorded rule the spec no longer asks for is pruned on the next reconcile. Set `Unmanaged` from the start if you want to own rules with those names yourself
- **GCPManagedCluster (GKE):** The `defaultRulesManagement` field is ignored. CAPG does not create default firewall rules for GKE clusters because the default rules target CAPI-specific instance tags (e.g. `<cluster>-control-plane`, `<cluster>-node`) that do not exist on GKE nodes. Only custom firewall rules defined in `firewallRules` are reconciled
- When using a shared VPC (`HostProject`), CAPG will not create, modify, or delete any firewall rules (default or custom)

## Custom Firewall Rules

You can define up to 50 additional firewall rules using the `firewallRules` field:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      defaultRulesManagement: "Managed"
      firewallRules:
        - name: "custom-ingress-rule"
          description: "Allow SSH and custom application traffic"
          direction: "Ingress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "22"
                - "8080"
                - "8443"
          sourceRanges:
            - "10.0.0.0/8"
            - "172.16.0.0/12"
          targetTags:
            - "web-servers"
```

### FirewallRule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Optional | Rule name (1-63 chars, must match `[a-z]([-a-z0-9]*[a-z0-9])?`). If not prefixed with cluster name, it will be prepended automatically. When omitted, the name is the cluster name followed by a suffix derived from the rule, truncated to 63 characters. |
| `description` | string | Optional | Description of the rule (max 2000 chars). Defaults to "Created by Cluster API GCP Provider". |
| `direction` | string | Optional | Traffic direction: `Ingress` (default) or `Egress`. |
| `priority` | integer | Optional | Rule priority (1-65535). Lower values = higher priority. Defaults to 1000 when omitted. |
| `allowed` | []FirewallDescriptor | Optional | List of ALLOW rules (max 1024). Cannot be set together with `denied`. |
| `denied` | []FirewallDescriptor | Optional | List of DENY rules (max 1024). Cannot be set together with `allowed`. |
| `sourceRanges` | []string | Optional | Source IP ranges in CIDR format (max 1024). Supports IPv4 and IPv6. Not valid for Egress rules. |
| `sourceTags` | []string | Optional | Source instance tags (max 30, 1-63 chars each). Only applies to traffic between instances in the same VPC. Not valid for Egress rules. |
| `destinationRanges` | []string | Optional | Destination IP ranges in CIDR format (max 1024). Only valid for Egress rules. |
| `targetTags` | []string | Optional | Target instance tags (max 70, 1-63 chars each). If empty, rule applies to all instances. |

### FirewallDescriptor Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `IPProtocol` | string | Yes | Protocol: `TCP`, `UDP`, `ICMP`, `ESP`, `AH`, `IPIP`, or `SCTP`. |
| `ports` | []string | Optional | Port numbers or ranges (e.g., `["22"]`, `["80","443"]`, `["12345-12349"]`). Only applicable for TCP/UDP. Max 500 entries. |

## Best Practices

1. **Use Descriptive Names**: Choose meaningful names that describe the rule's purpose
2. **Set Appropriate Priorities**: Use lower values (higher priority) for more critical security rules
3. **Minimize Source Ranges**: Restrict access to only necessary IP ranges
4. **Use Tags Strategically**: Leverage instance tags for flexible rule targeting
5. **Document Rules**: Always include a description explaining the rule's purpose
6. **Test Before Production**: Verify firewall rules in a development environment first
7. **Avoid Priority 65535**: GCP reserves this priority for implied rules
8. **Consider DENY Rules**: Use DENY rules for explicit blocking with higher priority

## Examples

### Example 1: Allow SSH from Specific IP Range

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      firewallRules:
        - name: "allow-ssh"
          description: "Allow SSH from office network"
          direction: "Ingress"
          priority: 900
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "22"
          sourceRanges:
            - "203.0.113.0/24"
          targetTags:
            - "ssh-enabled"
```

### Example 2: Allow Application Traffic

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      firewallRules:
        - name: "web-traffic"
          description: "Allow HTTP and HTTPS"
          direction: "Ingress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "80"
                - "443"
          sourceRanges:
            - "0.0.0.0/0"
          targetTags:
            - "web-servers"
```

### Example 3: Deny Rule with Higher Priority

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      firewallRules:
        - name: "deny-telnet"
          description: "Block telnet traffic"
          direction: "Ingress"
          priority: 500
          denied:
            - IPProtocol: "TCP"
              ports:
                - "23"
          sourceRanges:
            - "0.0.0.0/0"
```

### Example 4: Egress Rule for External Services

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      firewallRules:
        - name: "allow-external-api"
          description: "Allow egress to external API"
          direction: "Egress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "443"
          destinationRanges:
            - "198.51.100.0/24"
          targetTags:
            - "api-client"
```

### Example 5: Multiple Protocols and Ports

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: my-cluster
spec:
  network:
    firewall:
      firewallRules:
        - name: "multi-service"
          description: "Allow multiple services"
          direction: "Ingress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "80"
                - "443"
                - "8080-8090"
            - IPProtocol: "UDP"
              ports:
                - "53"
            - IPProtocol: "ICMP"
          sourceRanges:
            - "10.0.0.0/8"
          targetTags:
            - "multi-service-node"
```

### Example 6: GKE Managed Cluster with Firewall Rules

> **Note:** `defaultRulesManagement` is ignored for `GCPManagedCluster`. Only custom firewall rules in `firewallRules` are reconciled.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPManagedCluster
metadata:
  name: my-gke-cluster
spec:
  project: my-gcp-project
  region: us-central1
  network:
    name: gke-network
    firewall:
      firewallRules:
        - name: "allow-nodeport-range"
          description: "Allow NodePort traffic to GKE nodes"
          direction: "Ingress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "30000-32767"
          sourceRanges:
            - "10.0.0.0/8"
          targetTags:
            - "gke-node"
```

## Complete Example with Default Rule Management

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: GCPCluster
metadata:
  name: production-cluster
spec:
  project: my-gcp-project
  region: us-central1
  network:
    name: production-network
    firewall:
      # Manage default health check and cluster internal rules
      defaultRulesManagement: "Managed"
      # Add custom rules
      firewallRules:
        # Allow monitoring from Prometheus
        - name: "monitoring"
          description: "Allow Prometheus scraping"
          direction: "Ingress"
          priority: 900
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "9090"
                - "9100"
          sourceRanges:
            - "10.128.0.0/16"
          targetTags:
            - "prometheus-target"
        # Allow database access from application tier
        - name: "database-access"
          description: "Allow app to database communication"
          direction: "Ingress"
          priority: 1000
          allowed:
            - IPProtocol: "TCP"
              ports:
                - "5432"
                - "3306"
          sourceTags:
            - "app-tier"
          targetTags:
            - "database-tier"
```

## Troubleshooting

### Rules Not Applied

The controller logs each firewall rule it creates, updates, skips, or fails on, along with the reason. Start there:

```sh
kubectl -n capg-system logs deploy/capg-controller-manager | grep -i firewall
```

Failures are also recorded as Warning events on the cluster object (`kubectl describe gcpcluster my-cluster`).

If the logs don't explain it:

1. Check that `defaultRulesManagement` is set to `Managed` if you expect default rules
2. Verify you're not using a shared VPC (HostProject), which disables custom firewall rules
3. Ensure rule names are unique and follow GCP naming conventions. CAPG prepends the cluster name and truncates to 63 characters, so two long names can collide into one rule
4. Compare `spec.network.firewall.firewallRules` with `status.network.firewallRules`, which lists the rules CAPG manages
5. Check GCP Cloud Console for any conflicting VPC firewall rules

### Connection Issues

If experiencing connection problems:

1. Verify source/destination ranges include the correct IP addresses
2. Check that priority values don't conflict with DENY rules
3. Ensure target tags match your instance tags
4. Review that the correct protocol and ports are specified
5. Check GCP VPC firewall logs for blocked traffic

### Validation Errors

Common validation errors:

- **Invalid CIDR**: Ensure IP ranges use valid CIDR notation
- **Name too long**: Rule names are limited to 63 characters
- **Too many rules**: Maximum 50 custom rules per cluster
- **Invalid port format**: Use format like `"80"`, `"443"`, or `"8080-8090"`
