module sigs.k8s.io/cluster-api-provider-gcp/hack/tools

go 1.26.0

toolchain go1.26.8

tool (
	github.com/a8m/envsubst/cmd/envsubst
	gotest.tools/gotestsum
	sigs.k8s.io/cluster-api/hack/tools/conversion-verifier
	sigs.k8s.io/cluster-api/hack/tools/mdbook/embed
	sigs.k8s.io/cluster-api/hack/tools/mdbook/releaselink
	sigs.k8s.io/controller-runtime/tools/setup-envtest
)

require sigs.k8s.io/cluster-api/hack/tools v0.0.0-20260908123630-5f7d58c9ad64 // indirect

require (
	github.com/a8m/envsubst v1.4.2 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/dnephin/pflag v1.0.7 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.10.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gotest.tools/gotestsum v1.6.4 // indirect
	k8s.io/apiextensions-apiserver v0.36.3 // indirect
	k8s.io/apimachinery v0.36.3 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260427204847-8949caaa1199 // indirect
	k8s.io/utils v0.0.0-20260319190234-28399d86e0b5 // indirect
	sigs.k8s.io/cluster-api v1.11.0 // indirect
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.24.1 // indirect
	sigs.k8s.io/controller-tools v0.21.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kubebuilder/docs/book/utils v0.0.0-20260611053758-c72b289c1ec8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
