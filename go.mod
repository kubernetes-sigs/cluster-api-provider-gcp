module sigs.k8s.io/cluster-api-provider-gcp

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20210224082022-3d97a244fca7
	google.golang.org/api v0.20.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/component-base v0.21.1
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.3.11-0.20210528213424-a74b6a6428cb
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
)
