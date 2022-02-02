# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://www.thapaliya.com/en/writings/well-documented-makefiles/

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

GOPATH  := $(shell go env GOPATH)
GOARCH  := $(shell go env GOARCH)
GOOS    := $(shell go env GOOS)
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Default timeout for starting/stopping the Kubebuilder test control plane
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT ?=60s
export KUBEBUILDER_CONTROLPLANE_STOP_TIMEOUT ?=60s

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

# curl retries
CURL_RETRIES=3

# Directories.
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh
E2E_CONF_FILE ?= $(ROOT_DIR)/test/e2e/config/gcp-ci.yaml
E2E_CONF_FILE_ENVSUBST := $(ROOT_DIR)/test/e2e/config/gcp-ci-envsubst.yaml
E2E_DATA_DIR ?= $(ROOT_DIR)/test/e2e/data
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance.yaml)

# Binaries.
CLUSTERCTL := $(BIN_DIR)/clusterctl

CONTROLLER_GEN_VER := v0.7.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)

CONVERSION_GEN_VER := v0.22.2
CONVERSION_GEN_BIN := conversion-gen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/$(CONVERSION_GEN_BIN)-$(CONVERSION_GEN_VER)

ENVSUBST_VER := v1.2.0
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

GOLANGCI_LINT_VER := v1.43.0
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

KUSTOMIZE_VER := v3.8.6
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER)

RELEASE_NOTES_VER := v0.11.0
RELEASE_NOTES_BIN := release-notes
RELEASE_NOTES := $(TOOLS_BIN_DIR)/$(RELEASE_NOTES_BIN)-$(RELEASE_NOTES_VER)

GINKGO_VER := v1.16.4
GINKGO_BIN := ginkgo
GINKGO := $(TOOLS_BIN_DIR)/$(GINKGO_BIN)-$(GINKGO_VER)

KUBECTL_VER := v1.22.3
KUBECTL_BIN := kubectl
KUBECTL := $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)-$(KUBECTL_VER)

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Define Docker related variables. Releases should modify and double check these vars.
export GCP_PROJECT ?= $(shell gcloud config get-value project)
REGISTRY ?= gcr.io/$(GCP_PROJECT)
STAGING_REGISTRY := gcr.io/k8s-staging-cluster-api-gcp
PROD_REGISTRY := us.gcr.io/k8s-artifacts-prod/cluster-api-gcp
IMAGE_NAME ?= cluster-api-gcp-controller
export CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
export TAG ?= dev
export ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Hosts running SELinux need :z added to volume mounts
SELINUX_ENABLED := $(shell cat /sys/fs/selinux/enforce 2> /dev/null || echo 0)

ifeq ($(SELINUX_ENABLED),1)
  DOCKER_VOL_OPTS?=:z
endif

# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

GOLANG_VERSION := 1.16.9

# CI
CAPG_WORKER_CLUSTER_KUBECONFIG ?= "/tmp/kubeconfig"

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: ## Run tests
	source ./scripts/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v ./...

# Allow overriding the e2e configurations
GINKGO_FOCUS ?= Workload cluster creation
GINKGO_NODES ?= 3
GINKGO_NOCOLOR ?= false
GINKGO_ARGS ?=
ARTIFACTS ?= $(ROOT_DIR)/_artifacts
SKIP_CLEANUP ?= false
SKIP_CREATE_MGMT_CLUSTER ?= false

.PHONY: test-e2e-run
test-e2e-run: $(ENVSUBST) $(KUBECTL) $(GINKGO) e2e-image ## Run the end-to-end tests
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
	time $(GINKGO) -v -trace -progress -v -tags=e2e -focus=$(GINKGO_FOCUS) -nodes=$(GINKGO_NODES) --noColor=$(GINKGO_NOCOLOR) ./test/e2e -- \
		-e2e.artifacts-folder="$(ARTIFACTS)" \
		-e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
		-e2e.skip-resource-cleanup=$(SKIP_CLEANUP) \
		-e2e.use-existing-cluster=$(SKIP_CREATE_MGMT_CLUSTER) $(E2E_ARGS)

.PHONY: test-e2e
test-e2e: ## Run the end-to-end tests
	$(MAKE) test-e2e-run

CONFORMANCE_E2E_ARGS ?= -kubetest.config-file=$(KUBETEST_CONF_PATH)
CONFORMANCE_E2E_ARGS += $(E2E_ARGS)
.PHONY: test-conformance
test-conformance: ## Run conformance test on workload cluster.
	$(MAKE) test-e2e-run GINKGO_FOCUS="Conformance Tests" E2E_ARGS='$(CONFORMANCE_E2E_ARGS)' GINKGO_ARGS='$(LOCAL_GINKGO_ARGS)'

## --------------------------------------
## Binaries
## --------------------------------------

.PHONY: binaries
binaries: manager ## Builds and installs all binaries

.PHONY: manager
manager: ## Build manager binary.
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/manager .

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CLUSTERCTL): go.mod ## Build clusterctl binary.
	go build -o $(BIN_DIR)/clusterctl sigs.k8s.io/cluster-api/cmd/clusterctl

$(ENVSUBST): ## Build envsubst from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/a8m/envsubst/cmd/envsubst $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(GOLANGCI_LINT): ## Build golangci-lint from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(KUSTOMIZE): ## Build kustomize from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/kustomize/kustomize/v3 $(KUSTOMIZE_BIN) $(KUSTOMIZE_VER)

$(CONTROLLER_GEN): ## Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

$(CONVERSION_GEN): ## Build conversion-gen.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) k8s.io/code-generator/cmd/conversion-gen $(CONVERSION_GEN_BIN) $(CONVERSION_GEN_VER)

$(RELEASE_NOTES): ## Build release notes.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) k8s.io/release/cmd/release-notes $(RELEASE_NOTES_BIN) $(RELEASE_NOTES_VER)

$(GINKGO): ## Build ginkgo.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/onsi/ginkgo/ginkgo $(GINKGO_BIN) $(GINKGO_VER)

$(KUBECTL): ## Build kubectl
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(KUBECTL)*"
	curl --retry $(CURL_RETRIES) -fsL https://dl.k8s.io/release/$(KUBECTL_VER)/bin/$(GOOS)/$(GOARCH)/kubectl -o $(KUBECTL)
	ln -sf "$(KUBECTL)" "$(TOOLS_BIN_DIR)/$(KUBECTL_BIN)"
	chmod +x "$(TOOLS_BIN_DIR)/$(KUBECTL_BIN)" "$(KUBECTL)"


## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --fast=false

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring.
	go mod tidy

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(CONVERSION_GEN) ## Runs Go related generate targets
	$(CONTROLLER_GEN) \
		paths=./api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt
	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha3 \
		--build-tag=ignore_autogenerated_core_v1alpha3 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1alpha3 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha4 \
		--build-tag=ignore_autogenerated_core_v1alpha4 \
		--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1alpha4 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt $(OUTPUT_BASE)
	go generate ./...

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: e2e-image
e2e-image:
	docker build --build-arg LDFLAGS="$(LDFLAGS)" --tag=gcr.io/k8s-staging-cluster-api-gcp/cluster-api-gcp-controller:e2e .

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) $(MAKE) set-manifest-image
	$(MAKE) set-manifest-pull-policy

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' ./config/default/manager_image_patch.yaml

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' ./config/default/manager_pull_policy.yaml

## --------------------------------------
## Release
## --------------------------------------

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := out

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG)
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent
	$(MAKE) release-manifests
	$(MAKE) release-metadata
	$(MAKE) release-templates

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	cp templates/cluster-template* $(RELEASE_DIR)/

.PHONY: release-staging
release-staging: ## Builds and push container images to the staging bucket.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

RELEASE_ALIAS_TAG=$(PULL_BASE_REF)

.PHONY: release-alias-tag
release-alias-tag: # Adds the tag to the last build tag.
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)

.PHONY: release-notes
release-notes: $(RELEASE_NOTES)
	$(RELEASE_NOTES)

## --------------------------------------
## Development
## --------------------------------------

CLUSTER_NAME ?= test1

.PHONY: create-management-cluster
create-management-cluster: $(KUSTOMIZE) $(ENVSUBST)
	## Create kind management cluster.
	kind create cluster --name=clusterapi

	# Install cert manager and wait for availability
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.1.0/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager
	kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-webhook

	# Deploy CAPI
	curl --retry $(CURL_RETRIES) -sSL https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.0.4/cluster-api-components.yaml | $(ENVSUBST) | kubectl apply -f -

	# Deploy CAPG
	kind load docker-image $(CONTROLLER_IMG)-$(ARCH):$(TAG) --name=clusterapi
	$(KUSTOMIZE) build config/default | $(ENVSUBST) | kubectl apply -f -

	# Wait for CAPI pods
	kubectl wait --for=condition=Available --timeout=5m -n capi-system deployment -l cluster.x-k8s.io/provider=cluster-api
	kubectl wait --for=condition=Available --timeout=5m -n capi-kubeadm-bootstrap-system deployment -l cluster.x-k8s.io/provider=bootstrap-kubeadm
	kubectl wait --for=condition=Available --timeout=5m -n capi-kubeadm-control-plane-system deployment -l cluster.x-k8s.io/provider=control-plane-kubeadm

	# Wait for CAPG pods
	kubectl wait --for=condition=Ready --timeout=5m -n capg-system pod -l cluster.x-k8s.io/provider=infrastructure-gcp

	# required sleep for when creating management and workload cluster simultaneously
	sleep 10
	@echo 'Set kubectl context to the kind management cluster by running "kubectl config set-context kind-clusterapi"'

.PHONY: create-workload-cluster
create-workload-cluster: $(KUSTOMIZE) $(ENVSUBST)
	# Create workload Cluster.
	$(KUSTOMIZE) build templates | $(ENVSUBST) | kubectl apply -f -

	# Wait for the kubeconfig to become available.
	${TIMEOUT} 5m bash -c "while ! kubectl get secrets | grep $(CLUSTER_NAME)-kubeconfig; do sleep 1; done"
	# Get kubeconfig and store it locally.
	kubectl get secrets $(CLUSTER_NAME)-kubeconfig -o json | jq -r .data.value | base64 --decode > $(CAPG_WORKER_CLUSTER_KUBECONFIG)
	${TIMEOUT} 15m bash -c "while ! kubectl --kubeconfig=$(CAPG_WORKER_CLUSTER_KUBECONFIG) get nodes | grep master; do sleep 1; done"

	# Deploy calico
	kubectl --kubeconfig=$(CAPG_WORKER_CLUSTER_KUBECONFIG) apply -f https://docs.projectcalico.org/manifests/calico.yaml

	@echo 'run "kubectl --kubeconfig=$(CAPG_WORKER_CLUSTER_KUBECONFIG) ..." to work with the new target cluster'

.PHONY: create-cluster
create-cluster: create-management-cluster create-workload-cluster ## Create a development Kubernetes cluster on GCP in a KIND management cluster.

.PHONY: delete-workload-cluster
delete-workload-cluster: ## Deletes the example workload Kubernetes cluster
	@echo 'Your GCP resources will now be deleted, this can take up to 20 minutes'
	kubectl delete cluster $(CLUSTER_NAME)

.PHONY: kind-reset
kind-reset: ## Destroys the kind clusters.
	kind delete cluster --name=capg || true
	kind delete cluster --name=capg-e2e || true
	kind delete cluster --name=clusterapi || true

## --------------------------------------
## Tilt / Kind
## --------------------------------------

.PHONY: kind-create
kind-create: $(KUBECTL) ## create capg kind cluster if needed
	./scripts/kind-with-registry.sh

.PHONY: tilt-up
tilt-up: $(ENVSUBST) $(KUSTOMIZE) $(KUBECTL) kind-create ## start tilt and build kind cluster if needed
	EXP_CLUSTER_RESOURCE_SET=true tilt up

.PHONY: delete-cluster
delete-cluster: delete-workload-cluster  ## Deletes the example kind cluster "capg"
	kind delete cluster --name=capg

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin
	$(MAKE) clean-temporary

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders
	rm -f minikube.kubeconfig
	rm -f kubeconfig

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: verify
verify: verify-boilerplate verify-modules verify-gen

.PHONY: verify-boilerplate
verify-boilerplate:
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum); then \
		echo "go module files are out of date"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate
	@if !(git diff --quiet HEAD); then \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi
