---
title: Managed Kubernetes in CAPG
authors:
- "@richardchen331"
- "@richardcase"
reviewers: []
creation-date: 2022-11-23
last-updated: 2022-11-23
status: provisional
see-also: []
replaces: []
superseded-by: []
---

# Managed Kubernetes in CAPG

## Table of Contents

- [Managed Kubernetes in CAPG](#managed-kubernetes-in-capg)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
    - [Requirements](#requirements)
      - [Functional Requirements](#functional-requirements)
      - [Non-Functional Requirements](#non-functional-requirements)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Security Model](#security-model)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
  - [Implementation History](#implementation-history)

## Glossary

- **Managed Kubernetes** - a Kubernetes service offered/hosted by a service provider where the control plane is run & managed by the service provider. As a cluster service consumer, you don’t have to worry about managing/operating the control plane machines. Additionally, the managed Kubernetes service may extend to cover running managed worker nodes. Examples are GKE in GCP and EKS in AWS. This is different from a traditional implementation in Cluster API, where the control plane and worker nodes are deployed and managed by the cluster admin.
- **Managed Worker Node** - an individual Kubernetes worker node where the underlying compute (vm or bare-metal) is provisioned and managed by the service provider. This usually includes the joining of the newly provisioned node into a Managed Kubernetes cluster. The lifecycle is normally controlled via a higher level construct such as a Managed Node Group.
- **Managed Node Group** - is a service that a service provider offers that automates the provisioning of managed worker nodes. Depending on the service provider this group of nodes could contain a fixed number of replicas or it might contain a dynamic pool of replicas that auto-scales up and down. Examples are Node Pools in GCP and EKS managed node groups.
- **Cluster Infrastructure Provider (Infrastructure)** - an Infrastructure provider supplies whatever prerequisites are necessary for creating & running clusters such as networking, load balancers, firewall rules, and so on. ([docs](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/book/src/developer/providers/cluster-infrastructure.md))
- **ControlPlane Provider (ControlPlane)** - a control plane provider instantiates a Kubernetes control plane consisting of k8s control plane components such as kube-apiserver, etcd, kube-scheduler and kube-controller-manager. ([docs](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/book/src/developer/architecture/controllers/control-plane.md#control-plane-provider))
- **MachinePool (experimental)** - a MachinePool is similar to a MachineDeployment in that they both define configuration and policy for how a set of machines are managed. While the MachineDeployment uses MachineSets to orchestrate updates to the Machines, MachinePool delegates the responsibility to a cloud provider specific resource such as AWS Auto Scale Groups, GCP Managed Instance Groups, and Azure Virtual Machine Scale Sets. ([docs](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20190919-machinepool-api.md))
- [CAPI](https://github.com/kubernetes-sigs/cluster-api) - Cluster API.
- [CAPG](https://github.com/kubernetes-sigs/cluster-api-provider-gcp) - Cluster API Provider GCP.
- [CAPA](https://github.com/kubernetes-sigs/cluster-api-provider-aws) - Cluster API Provider AWS.
- [CAPZ](https://github.com/kubernetes-sigs/cluster-api-provider-azure) - Cluster API Provider AWS.

## Summary

Currently, CAPG only supports unmanaged Kubernetes clusters. This proposal outlines changes to CAPG that will introduce new capabilities to create managed Kubernetes clusters in GCP (GKE). This would bring CAPG inline with CAPA and CAPZ, both of which support creating unmanaged and managed clusters.

## Motivation

We are increasingly hearing requests from users of CAPG for GKE support (example [here](https://kubernetes.slack.com/archives/C01D1RFEN9G/p1644266939998709), [here](https://kubernetes.slack.com/archives/C01D1RFEN9G/p1661767000916779), [here](https://kubernetes.slack.com/archives/C01D1RFEN9G/p1666313627425529)), this is a major blocker for some users to onboard to CAPG.

The motivation is to add GKE support to CAPG to fulfill these user requests, and bring CAPG inline with CAPA and CAPZ, both of which support creating unmanaged and managed clusters.

### Goals

- New API to support the creation of GKE clusters
- New API to support the creation of GKE node pools

### Non-Goals/Future Work

- GKE ClusterClass support

## Proposal

At a high level, the plan is to:

1. Add a new `GCPManagedCluster` kind. This presents the properties needed to provision and manage the general GCP operating infrastructure for the cluster (i.e project, networking, iam). It would contain similar properties to `GCPCluster` and its reconciliation would be very similar.

2. Add a new `GCPManagedControlPlane` kind. This presents the actual GKE control plane in GCP. Its spec would only contain properties that are specific to the provisioning & management of a GKE cluster in GCP (excluding worker nodes). It would not contain any properties related to the general GCP operating infrastructure, like the networking or project.
3. Add a new `GCPManagedMachinePool` kind. This represents the GKE node pools in GCP. Its spec would contain properties that are specific to the provisioning & management of a GKE node pool in GCP.
4. Implement the reconciliation loop for `GCPManagedCluster`. It would be similar to the reconciliation of `GCPCluster`.
5. Implement the reconciliation loop for `GCPManagedControlPlane`. Initially, only Standard GKE cluster will be supported, and k8s version need to be specific in `GCPManagedControlPlane`.
6. Implement the reconciliation loop for `GCPManagedMachinePool`. Initially, k8s version and image type need to be specific in `GCPManagedMachinePool`. Later, we can add logic to discover the default k8s version from GCP.
7. Put GKE logic behind experimental feature gate.
8. Update controller IAM role permissions to enable calling GKE APIs.
9. Add cluster template for GKE.
10. Add e2e tests for GKE.
11. Add validation webhooks for `GCPManagedCluster`, `GCPManagedControlPlane`, and `GCPManagedMachinePool`.
12. Update any relevant documentation.

### User Stories

#### Story 1

AS a CAPG user
I want to use CAPG to provision and manage GKE clusters that utilize GCP’s managed Kubernetes service
So that I don’t have to worry about the management/provisioning of control plane nodes, and so I can take advantage of any value add services offered by GCP

#### Story 2

As a CAPG user
I want to use CAPG to provision and manage the lifecycle of node pools that utilize GCP’s managed instances
So that I don't have to worry about the management of these instances

### Requirements

#### Functional Requirements

**FR1:** CAPG MUST support creating GKE clusters.

**FR2:** CAPG MUST validate the input an user specifies for a GKE cluster.

#### Non-Functional Requirements

**NFR1:** CAPG MUST provide logging and tracing to expose the progress of reconciling the GKE cluster.

**NFR2:** CAPG MUST raise events at important milestones during reconciliation.

**NFR3:** CAPG MUST have e2e tests that cover usage of GKE.

### Implementation Details/Notes/Constraints

Proposed interfaces of `GCPManagedCluster`, `GCPManagedControlPlane`, and `GCPManagedMachinePool`:

```go
type GCPManagedClusterSpec struct {
    // Project is the name of the project to deploy the cluster to.
    Project string `json:"project"`

    // The GCP Region the cluster lives in.
    Region string `json:"region"`

    // ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
    // +optional
    ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

    // NetworkSpec encapsulates all things related to the GCP network.
    // +optional
    Network NetworkSpec `json:"network"`

    // AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
    // ones added by default.
    // +optional
    AdditionalLabels Labels `json:"additionalLabels,omitempty"`
}

type GCPManagedClusterStatus struct {
    FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
    Network        Network                  `json:"network,omitempty"`

    Ready bool `json:"ready"`

    // Conditions specifies the cpnditions for the managed control plane
    Conditions clusterv1.Conditions `json:”conditions,omitempty”`
}

type GCPManagedControlPlaneSpec struct {
    // EnableAutopilot indicates whether to enable autopilot for this GKE cluster
    EnableAutopilot bool `json:"enableAutopilot"`

    // Location represents the location (region or zone) in which the GKE cluster 
    // will be created
    Location string `json:"location"`

    // ReleaseChannel represents the release channel of the GKE cluster
    // +optional
    ReleaseChannel *string `json:"releaseChannel,omitempty"`

    // ControlPlaneVersion represents the control plane version of the GKE cluster
    // If not specified, the default version currently supported by GKE will be 
    // used
    // +optional
    ControlPlaneVersion *string `json:"controlPlaneVersion,omitempty"`

    // Endpoint represents the endpoint used to communicate with the control plane.
    Endpoint clusterv1.APIEndpoint `json:”endpoint”`
}

type GCPManagedControlPlaneStatus struct {
    Ready bool `json:"ready"`

    // Conditions specifies the cpnditions for the managed control plane
    Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

type GCPManagedMachinePoolSpec struct {
    // NodeVersion represents the node version of the node pool
    // If not specified, the GKE cluster control plane version will be used
    // +optional
    NodeVersion *string `json:"nodeVersion,omitempty"`

    // InitialNodeCount represents the initial number of nodes for the pool.
    // In regional or multi-zonal clusters, this is the number of nodes per zone.
    InitialNodeCount int32 `json:"initialNodeCount"`

    // KubernetesLabels specifies the labels to apply to the nodes of the node pool
    // +optional
    KubernetesLabels Labels `json:"kubernetesLabels,omitempty"`

    // KubernetesTaints specifies the taints to apply to the nodes of the node pool
    // +optional
    KubernetesTaints Taints `json:"kubernetesTaints,omitempty"`

    // AdditionalLabels is an optional set of tags to add to GCP resources managed by the GCP provider, in addition to the
    // ones added by default.
    // +optional
    AdditionalLabels Labels `json:"additionalLabels,omitempty"`

    // ProviderIDList are the provider IDs of instances in the
    // managed instance group corresponding to the nodegroup represented by this
    // machine pool
    // +optional
    ProviderIDList []string `json:"providerIDList,omitempty"`
}

type GCPManagedMachinePoolStatus struct {
    Ready bool `json:"ready"`

    // Replicas is the most recently observed number of replicas.
    // +optional
    Replicas int32 `json:"replicas"`

    // Conditions specifies the cpnditions for the managed machine pool
    Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}
```

### Security Model

We will need to add access to the new CRDs for the controllers.

GCP permission also needs to be updated so the controllers can call GKE APIs.

### Risks and Mitigations

The risk is that the implemented interface is different from CAPG and CAPZ. However it follows the recommended approach in the [Managed Kubernetes in CAPI](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220725-managed-kubernetes.md) proposal.

## Alternatives

Alternatives are to follow the other options discussed in the [Managed Kubernetes in CAPI](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220725-managed-kubernetes.md) proposal.

## Upgrade Strategy

We are adding new CRDs for GKE support. No existing CRDs are affected.

## Additional Details

### Test Plan

- E2e tests will need to be added for GKE.

## Implementation History

- [x] 2022-11-23: Initial WIP proposal created
