# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Image URL to use all building/pushing image targets
IMG ?= numaplane-controller
VERSION ?= latest
# BASE_VERSION will be used during release process to bump up versions
BASE_VERSION := latest
# Default cluster name where numaplane get deployed, update it as needed.
CLUSTER_NAME ?= staging-usw2-k8s
IMAGE_NAMESPACE ?= quay.io/numaproj
IMAGE_FULL_PATH ?= $(IMAGE_NAMESPACE)/$(IMG):$(VERSION)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.13.0

GCFLAGS="all=-N -l"

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0

# LOCAL GIT SERVER CONFIG
REPO_COUNT=5
GIT_SERVER_VERSION ?= latest
GITSERVER_IMAGE=quay.io/numaio/numaplane-e2e-gitserver:$(GIT_SERVER_VERSION)
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CURRENT_CONTEXT := $(shell [[ "`command -v kubectl`" != '' ]] && kubectl config current-context 2> /dev/null || echo "unset")
IMAGE_IMPORT_CMD := $(shell [[ "`command -v k3d`" != '' ]] && [[ "$(CURRENT_CONTEXT)" =~ k3d-* ]] && echo "k3d image import -c `echo $(CURRENT_CONTEXT) | cut -c 5-`")
ifndef IMAGE_IMPORT_CMD
IMAGE_IMPORT_CMD := $(shell [[ "`command -v minikube`" != '' ]] && [[ "$(CURRENT_CONTEXT)" =~ minikube* ]] && echo "minikube image load")
endif
ifndef IMAGE_IMPORT_CMD
IMAGE_IMPORT_CMD := $(shell [[ "`command -v kind`" != '' ]] && [[ "$(CURRENT_CONTEXT)" =~ kind-* ]] && echo "kind load docker-image")
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker
CONTAINER_TOOL:=$(shell command -v docker 2> /dev/null)
ifndef CONTAINER_TOOL
CONTAINER_TOOL:=$(shell command -v podman 2> /dev/null)
endif

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

##@ Development

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=config/crd/bases
	$(KUBECTL) kustomize config/default > config/install.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: codegen
codegen: generate manifests
	./hack/update-codegen.sh
	rm -rf ./vendor
	go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -race -v ./... -coverprofile cover.out

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: generate golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -gcflags=${GCFLAGS} -o bin/manager cmd/main.go

.PHONY: build-agent
build-agent: generate fmt vet ## Build agent binary.
	go build -gcflags=${GCFLAGS} -o bin/agent cmd/agent/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run -gcflags=${GCFLAGS} ./cmd/main.go

.PHONY: run-agent
run-agent: generate fmt vet ## Run agent from your host.
	go run -gcflags=${GCFLAGS} cmd/agent/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: image
image: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMAGE_FULL_PATH} .
ifdef IMAGE_IMPORT_CMD
	$(IMAGE_IMPORT_CMD) ${IMAGE_FULL_PATH}
endif

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMAGE_FULL_PATH}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMAGE_FULL_PATH=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMAGE_FULL_PATH=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMAGE_FULL_PATH} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: start
start: image
	$(KUBECTL) apply -f tests/manifests/numaplane-ns.yaml
	$(KUBECTL) kustomize tests/manifests | sed 's/CLUSTER_NAME_VALUE/$(CLUSTER_NAME)/g' | sed 's@quay.io/numaproj/@$(IMAGE_NAMESPACE)/@' | sed 's/$(IMG):$(BASE_VERSION)/$(IMG):$(VERSION)/' | $(KUBECTL) apply -f -

##@ Build Dependencies

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: gitserver
gitserver:
		cd tests/gitserver && \
		 docker buildx build \
		--no-cache \
		--build-arg REPO_COUNT=$(REPO_COUNT) \
		--platform $(PLATFORMS) \
		-t $(GITSERVER_IMAGE) \
		--push .

test-e2e:
test-%:
	kubectl -n numaplane-system delete pod localgitserver-0 --ignore-not-found=true
	kubectl apply -k ./tests/e2e-gitserver
	go generate $(shell find ./tests/$* -name '*.go')
	go test -v -timeout 15m -count 1 --tags test -p 1 ./tests/$*