# Copyright 2024 The Kubernetes Authors.
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

ROOT_DIR_RELATIVE := ../..

UNAME := $(shell uname -s)

OS := $(shell go env GOOS)
RUST_TARGET := unknown-$(OS)-gnu

ifeq ($(OS), darwin)
	RUST_TARGET := apple-darwin
	GH_ARCH_SUFFIX := macOS_amd64
	GTAR := gtar
endif

# Directories.
BIN_DIR := bin
BIN_DIR_ABS := $(abspath $(BIN_DIR))
SHARE_DIR := share

MDBOOK_EXTRACT_COMMAND := tar xfvz $(SHARE_DIR)/mdbook.tar.gz -C bin
MDBOOK_ARCHIVE_EXT := .tar.gz

MDBOOK_VERSION := v0.4.5

# Tooling binaries.
$(BIN_DIR):
	mkdir -p $@

$(SHARE_DIR):
	mkdir -p $@

# Binaries.
MDBOOK := $(BIN_DIR)/mdbook
MDBOOK_SHARE := $(SHARE_DIR)/mdbook$(MDBOOK_ARCHIVE_EXT)
$(MDBOOK): $(BIN_DIR) $(SHARE_DIR)
	curl -sL -o $(MDBOOK_SHARE) "https://github.com/rust-lang/mdBook/releases/download/$(MDBOOK_VERSION)/mdBook-$(MDBOOK_VERSION)-x86_64-$(RUST_TARGET)$(MDBOOK_ARCHIVE_EXT)"
	$(MDBOOK_EXTRACT_COMMAND)
	chmod +x $@
	touch -m $@

MDBOOK_EMBED := $(BIN_DIR)/mdbook-embed
$(MDBOOK_EMBED): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-embed sigs.k8s.io/cluster-api/hack/tools/mdbook/embed

MDBOOK_RELEASELINK := $(BIN_DIR)/mdbook-releaselink
$(MDBOOK_RELEASELINK): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-releaselink sigs.k8s.io/cluster-api/hack/tools/mdbook/releaselink

MDBOOK_TABULATE := $(BIN_DIR)/mdbook-tabulate
$(MDBOOK_TABULATE): $(BIN_DIR) go.mod go.sum
	go build -tags=tools -o $(BIN_DIR)/mdbook-tabulate sigs.k8s.io/cluster-api/hack/tools/mdbook/tabulate

.PHONY: clean
clean: ## Remove all tools
	rm -rf bin
	rm -rf share
