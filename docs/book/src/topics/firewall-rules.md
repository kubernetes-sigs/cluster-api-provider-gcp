# Firewall Rules

Cluster API Provider GCP allows you to configure GCP VPC firewall rules for your clusters through the `GCPCluster` resource. This feature provides fine-grained control over network access to your cluster infrastructure.

## Overview

Firewall rules are configured through the `network.firewall` field in the `GCPCluster` spec. The firewall configuration supports:

- **Default rule management**: Control whether the provider creates default firewall rules
- **Custom firewall rules**: Define additional firewall rules to meet your specific security requirements

## Default Firewall Rules

By default, the provider creates two firewall rules to enable cluster functionality:

1. **Health Check Rule** (`allow-<cluster-name>-healthchecks`):
   - Allows TCP traffic on port 6443 from GCP health check IP ranges
   - Source ranges: `35.191.0.0/16`, `130.211.0.0/22`
   - Target: control-plane nodes

2. **Cluster Internal Rule** (`allow-<cluster-name>-cluster`):
   - Allows all traffic between cluster nodes
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
- `Managed` (default): The controller creates and manages default firewall rules
- `Unmanaged`: The controller does not create or modify default firewall rules

**Important Notes:**
- Changing from `Managed` to `Unmanaged` after rules are created will not delete existing rules
- `defaultRulesManagement` has no effect when using a shared VPC (HostProject)

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
| `name` | string | Optional | Rule name (1-63 chars, must match `[a-z]([-a-z0-9]*[a-z0-9])?`). If not prefixed with cluster name, it will be prepended automatically. |
| `description` | string | Optional | Description of the rule (max 2000 chars). Defaults to "Created by Cluster API GCP Provider". |
| `direction` | string | Optional | Traffic direction: `Ingress` (default) or `Egress`. |
| `priority` | integer | Optional | Rule priority (1-65535). Lower values = higher priority. Default: 1000. |
| `allowed` | []FirewallDescriptor | Optional | List of ALLOW rules (max 1024). |
| `denied` | []FirewallDescriptor | Optional | List of DENY rules (max 1024). DENY rules take precedence over ALLOW rules with equal priority. |
| `sourceRanges` | []string | Optional | Source IP ranges in CIDR format (max 1024). Supports IPv4 and IPv6. |
| `sourceTags` | []string | Optional | Source instance tags (max 30, 1-63 chars each). Only applies to traffic between instances in the same VPC. |
| `destinationRanges` | []string | Optional | Destination IP ranges in CIDR format (max 1024). Only valid for Egress rules. |
| `targetTags` | []string | Optional | Target instance tags (max 70, 1-63 chars each). If empty, rule applies to all instances. |

### FirewallDescriptor Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `IPProtocol` | string | Yes | Protocol: `TCP`, `UDP`, `ICMP`, `ESP`, `AH`, `IPIP`, or `SCTP`. |
| `ports` | []string | Optional | Port numbers or ranges (e.g., `["22"]`, `["80","443"]`, `["12345-12349"]`). Only applicable for TCP/UDP. Max 500 entries. |

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
          sourceTags:
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

## Best Practices

1. **Use Descriptive Names**: Choose meaningful names that describe the rule's purpose
2. **Set Appropriate Priorities**: Use lower values for more critical security rules
3. **Minimize Source Ranges**: Restrict access to only necessary IP ranges
4. **Use Tags Strategically**: Leverage instance tags for flexible rule targeting
5. **Document Rules**: Always include a description explaining the rule's purpose
6. **Test Before Production**: Verify firewall rules in a development environment first
7. **Avoid Priority 65535**: GCP reserves this priority for implied rules
8. **Consider DENY Rules**: Use DENY rules for explicit blocking with higher priority

## Limitations

- Maximum 50 custom firewall rules per cluster
- Maximum 1024 allowed/denied descriptors per rule
- Maximum 500 ports per descriptor
- Maximum 1024 source/destination ranges per rule
- Maximum 30 source tags per rule
- Maximum 70 target tags per rule
- Rule names must be 1-63 characters and match `[a-z]([-a-z0-9]*[a-z0-9])?`
- Custom firewall rules have no effect when using a shared VPC (HostProject)
- For Egress rules, `sourceTags` cannot be specified

## Troubleshooting

### Rules Not Applied

If firewall rules are not being applied:

1. Check that `defaultRulesManagement` is set to `Managed` if you expect default rules
2. Verify you're not using a shared VPC (HostProject), which disables custom firewall rules
3. Ensure rule names are unique and follow GCP naming conventions
4. Check GCP Cloud Console for any conflicting VPC firewall rules

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
