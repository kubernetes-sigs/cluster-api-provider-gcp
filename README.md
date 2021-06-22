<p align="center"><img alt="capi" src="https://github.com/kubernetes-sigs/cluster-api/raw/master/docs/book/src/images/introduction.png" width="160x" /><img alt="capi" src="https://cloud.google.com/_static/cloud/images/favicons/onecloud/super_cloud.png" width="192x" /></p>
<p align="center"><a href="https://prow.k8s.io/?job=ci-cluster-api-provider-gcp-build">
<!-- prow build badge, godoc, and go report card-->
<img alt="Build Status" src="https://prow.k8s.io/badge.svg?jobs=ci-cluster-api-provider-gcp">
</a> <a href="https://godoc.org/sigs.k8s.io/cluster-api-provider-gcp"><img src="https://godoc.org/sigs.k8s.io/cluster-api-provider-gcp?status.svg"></a> <a href="https://goreportcard.com/report/sigs.k8s.io/cluster-api-provider-gcp"><img alt="Go Report Card" src="https://goreportcard.com/badge/sigs.k8s.io/cluster-api-provider-gcp" /></a></p>

# Kubernetes Cluster API Provider GCP

Kubernetes-native declarative infrastructure for GCP.
test
## What is the Cluster API?

The Cluster API is a Kubernetes project to bring declarative, Kubernetes-style
APIs to cluster creation, configuration, and management. It provides optional,
additive functionality on top of core Kubernetes.


## Community, discussion, contribution, and support

- Chat with us on [Slack](http://slack.k8s.io/) in the _#cluster-api_ channel
- Join the [SIG Cluster Lifecycle](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars
- Join our Cluster API working group sessions
    - Weekly on Wednesdays @ 10:00 PT on [Zoom][zoomMeeting]
    - Previous meetings: \[ [notes][notes] | [recordings][recordings] \]
- Provider implementers office hours
    - Weekly on Tuesdays @ 12:00 PT ([Zoom](providerZoomMeetingTues)) and Wednesdays @ 15:00 CET ([Zoom](providerZoomMeetingWed))
    - Previous meetings: \[ [notes][implementerNotes] \]

Pull Requests are very welcome!
See the [issue tracker] if you're unsure where to start, or feel free to reach out to discuss.

See also: our own [contributor guide] and the Kubernetes [community page].

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct].

[community page]: https://kubernetes.io/community
[Kubernetes Code of Conduct]: code-of-conduct.md
[notes]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY/edit
[recordings]: https://www.youtube.com/playlist?list=PL69nYSiGNLP29D0nYgAGWt1ZFqS9Z7lw4
[zoomMeeting]: https://zoom.us/j/861487554
[implementerNotes]: https://docs.google.com/document/d/1IZ2-AZhe4r3CYiJuttyciS7bGZTTx4iMppcA8_Pr3xE/edit
[providerZoomMeetingTues]: https://zoom.us/j/140808484
[providerZoomMeetingWed]: https://zoom.us/j/424743530
[issue tracker]: https://github.com/kubernetes-sigs/cluster-api/issues
[contributor guide]: CONTRIBUTING.md
