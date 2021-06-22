module sigs.k8s.io/cluster-api-provider-gcp

go 1.16

require (
	github.com/GoogleCloudPlatform/k8s-cloud-provider v1.16.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/mod v0.4.2
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
	google.golang.org/api v0.48.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/component-base v0.21.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0-beta.1
	sigs.k8s.io/cluster-api/test v0.4.0-beta.1
	sigs.k8s.io/controller-runtime v0.9.0
)

replace (
	github.com/GoogleCloudPlatform/k8s-cloud-provider => github.com/GoogleCloudPlatform/k8s-cloud-provider v1.16.1-0.20210622065854-abbfeadc9fda
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.0-beta.1
)
