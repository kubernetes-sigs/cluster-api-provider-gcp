<p align="center"><img alt="capi" src="https://github.com/kubernetes-sigs/cluster-api/raw/master/docs/book/src/images/introduction.png" width="160x" /><img alt="capi" src="https://cloud.google.com/_static/cloud/images/favicons/onecloud/super_cloud.png" width="192x" /></p>
<p align="center"><a href="https://prow.k8s.io/?job=ci-cluster-api-provider-gcp-build">
<!-- prow build badge, godoc, and go report card-->
<img alt="Build Status" src="https://prow.k8s.io/badge.svg?jobs=ci-cluster-api-provider-gcp">
</a> <a href="https://godoc.org/sigs.k8s.io/cluster-api-provider-gcp"><img src="https://godoc.org/sigs.k8s.io/cluster-api-provider-gcp?status.svg"></a> <a href="https://goreportcard.com/report/sigs.k8s.io/cluster-api-provider-gcp"><img alt="Go Report Card" src="https://goreportcard.com/badge/sigs.k8s.io/cluster-api-provider-gcp" /></a></p>

# Kubernetes Cluster API Provider GCP

Kubernetes-native declarative infrastructure for GCP.

## What is the Cluster API Provider GCP?

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings declarative Kubernetes-style APIs to cluster creation, configuration and management. The API itself is shared across multiple cloud providers allowing for true Google Cloud hybrid deployments of Kubernetes.

## Quick Start

Checkout our [Cluster API Quick Start] to create your first Kubernetes cluster
on Google Cloud Platform using Cluster API.

## Documentation

Presently our docs can be found [here](https://github.com/kubernetes-sigs/cluster-api-provider-gcp/tree/main/docs).

## Getting Involved and Contributing

Are you interested in contributing to cluster-api-provider-gcp? We, the maintainers 
and the community, would love your suggestions, support and contributions! The maintainers
of the project can be contacted anytime to learn about how to get involved.

Before starting with the contribution, please go through [prerequisites] of the project.

To set up the development environement checkout the [development guide].

In the interest of getting new people involved we have issues marked as [`good first issue`][good_first_issue]. Although
this issues have a smaller scope but are very helpful in getting acquainted with the codebase.
For more see the [issue tracker]. If you're unsure where to start, feel free to reach out to discuss.

See also: Our own [contributor guide] and the Kubernetes [community page].

We also encourage ALL active community participants to act as if they are maintainers, even if you don't have
'official' written permissions. This is a community effort and we are here to serve the Kubernetes community.
If you have an active interest and you want to get involved, you have real power!


### Office hours

- Join the [SIG Cluster Lifecycle](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars.
- Provider implementers office hours (CAPI)
    - Weekly on Tuesdays @ 12:00 PT ([Zoom](providerZoomMeetingTues)) and Wednesdays @ 15:00 CET ([Zoom](providerZoomMeetingWed))
    - Previous meetings: \[ [notes][implementerNotes] \]

### Other ways to communicate with the contributors

Please check in with us in the [#cluster-api-gcp] on Slack. 

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct].

[Cluster API Quick Start]: https://cluster-api.sigs.k8s.io/user/quick-start.html
[prerequisites]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/docs/prerequisites.md
[development guide]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/blob/main/docs/development.md
[good_first_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22
[issue tracker]: https://github.com/kubernetes-sigs/cluster-api-provider-gcp/issues
[contributor guide]: CONTRIBUTING.md 
[community page]: https://kubernetes.io/community
[Kubernetes Code of Conduct]: code-of-conduct.md
[providerZoomMeetingTues]: https://zoom.us/j/140808484
[providerZoomMeetingWed]: https://zoom.us/j/424743530
[#cluster-api-gcp]: https://sigs.k8s.io/cluster-api-provider-gcp
