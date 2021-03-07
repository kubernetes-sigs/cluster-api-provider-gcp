module sigs.k8s.io/cluster-api-provider-gcp

go 1.15

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.10.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	google.golang.org/api v0.20.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/cluster-api v0.3.11-0.20210209200458-51a6d64d171c
	sigs.k8s.io/controller-runtime v0.8.1
)
