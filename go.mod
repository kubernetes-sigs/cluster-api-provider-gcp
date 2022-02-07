module sigs.k8s.io/cluster-api-provider-gcp

go 1.16

require (
	github.com/GoogleCloudPlatform/k8s-cloud-provider v1.18.0
	github.com/google/go-cmp v0.5.6
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/mod v0.5.1
	golang.org/x/net v0.0.0-20211208012354-db4efeb81f4b
	google.golang.org/api v0.62.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/klog/v2 v2.10.0
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704
	sigs.k8s.io/cluster-api v1.0.4
	sigs.k8s.io/cluster-api/test v1.0.4
	sigs.k8s.io/controller-runtime v0.10.3
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.4
