module sigs.k8s.io/cluster-api-provider-gcp

go 1.16

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	google.golang.org/api v0.20.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/component-base v0.21.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0-beta.1
	sigs.k8s.io/cluster-api/test v0.4.0-beta.1
	sigs.k8s.io/controller-runtime v0.9.0
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.0-beta.1
