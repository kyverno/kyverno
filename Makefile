.DEFAULT_GOAL: build-all

############
# DEFAULTS #
############

GIT_VERSION          := $(shell git describe --match "v[0-9]*" --tags $(git rev-list --tags --max-count=1))
GIT_VERSION_DEV      := $(shell git describe --match "[0-9].[0-9]-dev*")
GIT_BRANCH           := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH             := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP            := $(shell date '+%Y-%m-%d_%I:%M:%S%p')
VERSION              ?= $(shell git describe --match "v[0-9]*")
REGISTRY             ?= ghcr.io
REPO                 ?= kyverno
IMAGE_TAG_LATEST_DEV  = $(shell git describe --match "[0-9].[0-9]-dev*" | cut -d '-' -f-2)
IMAGE_TAG_DEV         = $(GIT_VERSION_DEV)
IMAGE_TAG            ?= $(GIT_VERSION)
K8S_VERSION          ?= $(shell kubectl version --short | grep -i server | cut -d" " -f3 | cut -c2-)
KIND_IMAGE           ?= kindest/node:v1.24.4
KIND_NAME            ?= kind
GOOS                 ?= $(shell go env GOOS)
GOARCH               ?= $(shell go env GOARCH)
KOCACHE              ?= /tmp/ko-cache
BUILD_WITH           ?= ko
KYVERNOPRE_IMAGE     := kyvernopre
KYVERNO_IMAGE        := kyverno
CLI_IMAGE            := kyverno-cli
REPO_KYVERNOPRE      := $(REGISTRY)/$(REPO)/$(KYVERNOPRE_IMAGE)
REPO_KYVERNO         := $(REGISTRY)/$(REPO)/$(KYVERNO_IMAGE)
REPO_CLI             := $(REGISTRY)/$(REPO)/$(CLI_IMAGE)

#########
# TOOLS #
#########

TOOLS_DIR                          := $(PWD)/.tools
KIND                               := $(TOOLS_DIR)/kind
KIND_VERSION                       := v0.14.0
CONTROLLER_GEN                     := $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION             := v0.10.0
CLIENT_GEN                         := $(TOOLS_DIR)/client-gen
LISTER_GEN                         := $(TOOLS_DIR)/lister-gen
INFORMER_GEN                       := $(TOOLS_DIR)/informer-gen
OPENAPI_GEN                        := $(TOOLS_DIR)/openapi-gen
CODE_GEN_VERSION                   := v0.25.2
GEN_CRD_API_REFERENCE_DOCS         := $(TOOLS_DIR)/gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS_VERSION := latest
GO_ACC                             := $(TOOLS_DIR)/go-acc
GO_ACC_VERSION                     := latest
KUSTOMIZE                          := $(TOOLS_DIR)/kustomize
KUSTOMIZE_VERSION                  := latest
GOIMPORTS                          := $(TOOLS_DIR)/goimports
GOIMPORTS_VERSION                  := latest
HELM                               := $(TOOLS_DIR)/helm
HELM_VERSION                       := v3.10.1
HELM_DOCS                          := $(TOOLS_DIR)/helm-docs
HELM_DOCS_VERSION                  := v1.11.0
KO                                 := $(TOOLS_DIR)/ko
KO_VERSION                         := main #e93dbee8540f28c45ec9a2b8aec5ef8e43123966
KUTTL                              := $(TOOLS_DIR)/kubectl-kuttl
KUTTL_VERSION                      := v0.14.0
TOOLS                              := $(KIND) $(CONTROLLER_GEN) $(CLIENT_GEN) $(LISTER_GEN) $(INFORMER_GEN) $(OPENAPI_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GO_ACC) $(KUSTOMIZE) $(GOIMPORTS) $(HELM) $(HELM_DOCS) $(KO) $(KUTTL)
ifeq ($(GOOS), darwin)
SED                                := gsed
else
SED                                := sed
endif

$(KIND):
	@echo Install kind... >&2
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kind@$(KIND_VERSION)

$(CONTROLLER_GEN):
	@echo Install controller-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(CLIENT_GEN):
	@echo Install client-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/client-gen@$(CODE_GEN_VERSION)

$(LISTER_GEN):
	@echo Install lister-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/lister-gen@$(CODE_GEN_VERSION)

$(INFORMER_GEN):
	@echo Install informer-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/informer-gen@$(CODE_GEN_VERSION)

$(OPENAPI_GEN):
	@echo Install openapi-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/openapi-gen@$(CODE_GEN_VERSION)

$(GEN_CRD_API_REFERENCE_DOCS):
	@echo Install gen-crd-api-reference-docs... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

$(GO_ACC):
	@echo Install go-acc... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/ory/go-acc@$(GO_ACC_VERSION)

$(KUSTOMIZE):
	@echo Install kustomize... >&2
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION)

$(GOIMPORTS):
	@echo Install goimports... >&2
	@GOBIN=$(TOOLS_DIR) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

$(HELM):
	@echo Install helm... >&2
	@GOBIN=$(TOOLS_DIR) go install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION)

$(HELM_DOCS):
	@echo Install helm-docs... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

$(KO):
	@echo Install ko... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/google/ko@$(KO_VERSION)

$(KUTTL):
	@echo Install kuttl... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/kudobuilder/kuttl/cmd/kubectl-kuttl@$(KUTTL_VERSION)

.PHONY: install-tools
install-tools: $(TOOLS) ## Install tools

.PHONY: clean-tools
clean-tools: ## Remove installed tools
	@echo Clean tools... >&2
	@rm -rf $(TOOLS_DIR)

#################
# BUILD (LOCAL) #
#################

CMD_DIR        := ./cmd
KYVERNO_DIR    := $(CMD_DIR)/kyverno
KYVERNOPRE_DIR := $(CMD_DIR)/initContainer
CLI_DIR        := $(CMD_DIR)/cli/kubectl-kyverno
KYVERNO_BIN    := $(KYVERNO_DIR)/kyverno
KYVERNOPRE_BIN := $(KYVERNOPRE_DIR)/kyvernopre
CLI_BIN        := $(CLI_DIR)/kubectl-kyverno
PACKAGE        ?= github.com/kyverno/kyverno
CGO_ENABLED    ?= 0 
LD_FLAGS        = "-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"
LD_FLAGS_DEV    = "-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION_DEV) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"

.PHONY: fmt
fmt: ## Run go fmt
	@echo Go fmt... >&2
	@go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo Go vet... >&2
	@go vet ./...

.PHONY: unused-package-check
unused-package-check:
	@tidy=$$(go mod tidy); \
	if [ -n "$${tidy}" ]; then \
		echo "go mod tidy checking failed!"; echo "$${tidy}"; echo; \
	fi

$(KYVERNOPRE_BIN): fmt vet
	@echo Build kyvernopre binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(KYVERNOPRE_BIN) -ldflags=$(LD_FLAGS) $(KYVERNOPRE_DIR)

$(KYVERNO_BIN): fmt vet
	@echo Build kyverno binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(KYVERNO_BIN) -ldflags=$(LD_FLAGS) $(KYVERNO_DIR)

$(CLI_BIN): fmt vet
	@echo Build cli binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(CLI_BIN) -ldflags=$(LD_FLAGS) $(CLI_DIR)

.PHONY: build-kyvernopre
build-kyvernopre: $(KYVERNOPRE_BIN) ## Build kyvernopre binary

.PHONY: build-kyverno
build-kyverno: $(KYVERNO_BIN) ## Build kyverno binary

.PHONY: build-cli
build-cli: $(CLI_BIN) ## Build cli binary

build-all: build-kyvernopre build-kyverno build-cli ## Build all binaries

##############
# BUILD (KO) #
##############

PLATFORMS           := linux/amd64,linux/arm64,linux/s390x
LOCAL_PLATFORM      := linux/$(GOARCH)
KO_TAGS             := latest,$(IMAGE_TAG)
KO_TAGS_DEV         := latest,$(IMAGE_TAG_DEV)

.PHONY: ko-build-kyvernopre
ko-build-kyvernopre: $(KO) ## Build kyvernopre local image (with ko)
	@echo Build kyvernopre local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=ko.local $(KO) build $(KYVERNOPRE_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-kyverno
ko-build-kyverno: $(KO) ## Build kyverno local image (with ko)
	@echo Build kyverno local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=ko.local $(KO) build $(KYVERNO_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-cli
ko-build-cli: $(KO) ## Build cli local image (with ko)
	@echo Build cli local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=ko.local $(KO) build $(CLI_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-all
ko-build-all: ko-build-kyvernopre ko-build-kyverno ko-build-cli ## Build all local images (with ko)

################
# PUBLISH (KO) #
################

REGISTRY_USERNAME   ?= dummy
KO_KYVERNOPRE_IMAGE := ko.local/github.com/kyverno/kyverno/cmd/initcontainer
KO_KYVERNO_IMAGE    := ko.local/github.com/kyverno/kyverno/cmd/kyverno

.PHONY: ko-login
ko-login: $(KO)
	@$(KO) login $(REGISTRY) --username $(REGISTRY_USERNAME) --password $(REGISTRY_PASSWORD)

.PHONY: ko-publish-kyvernopre
ko-publish-kyvernopre: ko-login ## Build and publish kyvernopre image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNOPRE) $(KO) build $(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-kyverno
ko-publish-kyverno: ko-login ## Build and publish kyverno image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNO) $(KO) build $(KYVERNO_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-cli
ko-publish-cli: ko-login ## Build and publish cli image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLI) $(KO) build $(CLI_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-kyvernopre-dev
ko-publish-kyvernopre-dev: ko-login ## Build and publish kyvernopre dev image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNOPRE) $(KO) build $(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(PLATFORMS)

.PHONY: ko-publish-kyverno-dev
ko-publish-kyverno-dev: ko-login ## Build and publish kyverno dev image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNO) $(KO) build $(KYVERNO_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(PLATFORMS)

.PHONY: ko-publish-cli-dev
ko-publish-cli-dev: ko-login ## Build and publish cli dev image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLI) $(KO) build $(CLI_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(PLATFORMS)

.PHONY: ko-publish-all
ko-publish-all: ko-publish-kyvernopre ko-publish-kyverno ko-publish-cli ## Build and publish all images (with ko)

.PHONY: ko-publish-all-dev
ko-publish-all-dev: ko-publish-kyvernopre-dev ko-publish-kyverno-dev ko-publish-cli-dev ## Build and publish all dev images (with ko)

##################
# UTILS (DOCKER) #
##################

.PHONY: docker-get-kyvernopre-digest
docker-get-kyvernopre-digest: ## Get kyvernopre image digest (with docker)
	@docker buildx imagetools inspect --raw $(REPO_KYVERNOPRE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-kyvernopre-digest-dev
docker-get-kyvernopre-digest-dev: ## Get kyvernopre dev image digest (with docker)
	@docker buildx imagetools inspect --raw $(REPO_KYVERNOPRE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-kyverno-digest
docker-get-kyverno-digest: ## Get kyverno image digest (with docker)
	@docker buildx imagetools inspect --raw $(REPO_KYVERNO):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-kyverno-digest-dev
docker-get-kyverno-digest-dev: ## Get kyverno dev image digest (with docker)
	@docker buildx imagetools inspect --raw $(REPO_KYVERNO):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-buildx-builder
docker-buildx-builder:
	if ! docker buildx ls | grep -q kyverno; then\
		docker buildx create --name kyverno --use;\
	fi

##################
# BUILD (DOCKER) #
##################

DOCKER_KYVERNOPRE_IMAGE := $(REPO_KYVERNOPRE)
DOCKER_KYVERNO_IMAGE    := $(REPO_KYVERNO)

.PHONY: docker-build-kyvernopre
docker-build-kyvernopre: docker-buildx-builder ## Build kyvernopre local image (with docker)
	@echo Build kyvernopre local image with docker... >&2
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plain --load --platform $(LOCAL_PLATFORM) --tag $(REPO_KYVERNOPRE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-build-kyverno
docker-build-kyverno: docker-buildx-builder ## Build kyverno local image (with docker)
	@echo Build kyverno local image with docker... >&2
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plain --load --platform $(LOCAL_PLATFORM) --tag $(REPO_KYVERNO):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-build-cli
docker-build-cli: docker-buildx-builder ## Build cli local image (with docker)
	@echo Build cli local image with docker... >&2
	@docker buildx build --file $(CLI_DIR)/Dockerfile --progress plain --load --platform $(LOCAL_PLATFORM) --tag $(REPO_CLI):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-build-all
docker-build-all: docker-build-kyvernopre docker-build-kyverno docker-build-cli ## Build all local images (with docker)

####################
# PUBLISH (DOCKER) #
####################

.PHONY: docker-publish-kyvernopre
docker-publish-kyvernopre: docker-buildx-builder ## Build and publish kyvernopre image (with docker)
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) --tag $(REPO_KYVERNOPRE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-publish-kyvernopre-dev
docker-publish-kyvernopre-dev: docker-buildx-builder ## Build and publish kyvernopre dev image (with docker)
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) \
		--tag $(REPO_KYVERNOPRE):$(IMAGE_TAG_DEV) --tag $(REPO_KYVERNOPRE):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO_KYVERNOPRE):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-publish-kyverno
docker-publish-kyverno: docker-buildx-builder ## Build and publish kyverno image (with docker)
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) --tag $(REPO_KYVERNO):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-publish-kyverno-dev
docker-publish-kyverno-dev: docker-buildx-builder ## Build and publish kyverno dev image (with docker)
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) \
		--tag $(REPO_KYVERNO):$(IMAGE_TAG_DEV) --tag $(REPO_KYVERNO):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO_KYVERNO):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-publish-cli
docker-publish-cli: docker-buildx-builder ## Build and publish cli image (with docker)
	@docker buildx build --file $(CLI_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) --tag $(REPO_CLI):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-publish-cli-dev
docker-publish-cli-dev: docker-buildx-builder ## Build and publish cli dev image (with docker)
	@docker buildx build --file $(CLI_DIR)/Dockerfile --progress plain --push --platform $(PLATFORMS) \
		--tag $(REPO_CLI):$(IMAGE_TAG_DEV) --tag $(REPO_CLI):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO_CLI):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-publish-all
docker-publish-all: docker-publish-kyvernopre docker-publish-kyverno docker-publish-cli ## Build and publish all images (with docker)

.PHONY: docker-publish-all-dev
docker-publish-all-dev: docker-publish-kyvernopre-dev docker-publish-kyverno-dev docker-publish-cli-dev ## Build and publish all dev images (with docker)

#################
# BUILD (IMAGE) #
#################

LOCAL_KYVERNOPRE_IMAGE := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_KYVERNOPRE_IMAGE)
LOCAL_KYVERNO_IMAGE    := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_KYVERNO_IMAGE)

.PHONY: image-build-kyvernopre
image-build-kyvernopre: $(BUILD_WITH)-build-kyvernopre

.PHONY: image-build-kyverno
image-build-kyverno: $(BUILD_WITH)-build-kyverno

.PHONY: image-build-cli
image-build-cli: $(BUILD_WITH)-build-cli

.PHONY: image-build-all
image-build-all: $(BUILD_WITH)-build-all

###########
# CODEGEN #
###########

GOPATH_SHIM        := ${PWD}/.gopath
PACKAGE_SHIM       := $(GOPATH_SHIM)/src/$(PACKAGE)
OUT_PACKAGE        := $(PACKAGE)/pkg/client
INPUT_DIRS         := $(PACKAGE)/api/kyverno/v1,$(PACKAGE)/api/kyverno/v1beta1,$(PACKAGE)/api/kyverno/v1alpha2,$(PACKAGE)/api/policyreport/v1alpha2
CLIENTSET_PACKAGE  := $(OUT_PACKAGE)/clientset
LISTERS_PACKAGE    := $(OUT_PACKAGE)/listers
INFORMERS_PACKAGE  := $(OUT_PACKAGE)/informers

$(GOPATH_SHIM):
	@echo Create gopath shim... >&2
	@mkdir -p $(GOPATH_SHIM)

.INTERMEDIATE: $(PACKAGE_SHIM)
$(PACKAGE_SHIM): $(GOPATH_SHIM)
	@echo Create package shim... >&2
	@mkdir -p $(GOPATH_SHIM)/src/github.com/kyverno && ln -s -f ${PWD} $(PACKAGE_SHIM)

.PHONY: codegen-client-clientset
codegen-client-clientset: $(PACKAGE_SHIM) $(CLIENT_GEN) ## Generate clientset
	@echo Generate clientset... >&2
	@GOPATH=$(GOPATH_SHIM) $(CLIENT_GEN) --go-header-file ./scripts/boilerplate.go.txt --clientset-name versioned --output-package $(CLIENTSET_PACKAGE) --input-base "" --input $(INPUT_DIRS)

.PHONY: codegen-client-listers
codegen-client-listers: $(PACKAGE_SHIM) $(LISTER_GEN) ## Generate listers
	@echo Generate listers... >&2
	@GOPATH=$(GOPATH_SHIM) $(LISTER_GEN) --go-header-file ./scripts/boilerplate.go.txt --output-package $(LISTERS_PACKAGE) --input-dirs $(INPUT_DIRS)

.PHONY: codegen-client-informers
codegen-client-informers: $(PACKAGE_SHIM) $(INFORMER_GEN) ## Generate informers
	@echo Generate informers... >&2
	@GOPATH=$(GOPATH_SHIM) $(INFORMER_GEN) --go-header-file ./scripts/boilerplate.go.txt --output-package $(INFORMERS_PACKAGE) --input-dirs $(INPUT_DIRS) --versioned-clientset-package $(CLIENTSET_PACKAGE)/versioned --listers-package $(LISTERS_PACKAGE)

.PHONY: codegen-client-all
codegen-client-all: codegen-client-clientset codegen-client-listers codegen-client-informers ## Generate clientset, listers and informers

.PHONY: codegen-crds-kyverno
codegen-crds-kyverno: $(CONTROLLER_GEN) ## Generate kyverno CRDs
	@echo Generate kyverno crds... >&2
	@$(CONTROLLER_GEN) crd paths=./api/kyverno/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: codegen-crds-report
codegen-crds-report: $(CONTROLLER_GEN) ## Generate policy reports CRDs
	@echo Generate policy reports crds... >&2
	@$(CONTROLLER_GEN) crd paths=./api/policyreport/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: codegen-crds-all
codegen-crds-all: codegen-crds-kyverno codegen-crds-report ## Generate all CRDs

.PHONY: codegen-deepcopy-kyverno
codegen-deepcopy-kyverno: $(CONTROLLER_GEN) $(GOIMPORTS) ## Generate kyverno deep copy functions
	@echo Generate kyverno deep copy functions... >&2
	@$(CONTROLLER_GEN) object:headerFile="scripts/boilerplate.go.txt" paths="./api/kyverno/..." && $(GOIMPORTS) -w ./api/kyverno

.PHONY: codegen-deepcopy-report
codegen-deepcopy-report: $(CONTROLLER_GEN) $(GOIMPORTS) ## Generate policy reports deep copy functions
	@echo Generate policy reports deep copy functions... >&2
	@$(CONTROLLER_GEN) object:headerFile="scripts/boilerplate.go.txt" paths="./api/policyreport/..." && $(GOIMPORTS) -w ./api/policyreport

.PHONY: codegen-deepcopy-all
codegen-deepcopy-all: codegen-deepcopy-kyverno codegen-deepcopy-report ## Generate all deep copy functions

.PHONY: codegen-api-docs
codegen-api-docs: $(PACKAGE_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) ## Generate API docs
	@echo Generate api docs... >&2
	@rm -rf docs/user/crd && mkdir -p docs/user/crd
	@GOPATH=$(GOPATH_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) -v 4 \
			-api-dir github.com/kyverno/kyverno/api \
			-config docs/user/config.json \
			-template-dir docs/user/template \
			-out-file docs/user/crd/index.html

.PHONY: codegen-helm-docs
codegen-helm-docs: ## Generate helm docs
	@echo Generate helm docs... >&2
	@docker run -v ${PWD}/charts:/work -w /work jnorwood/helm-docs:v1.11.0 -s file

.PHONY: codegen-helm-crds
codegen-helm-crds: $(KUSTOMIZE) codegen-crds-all ## Generate helm CRDs
	@echo Create temp folder for kustomization... >&2
	@mkdir -p config/.helm
	@echo Create kustomization... >&2
	@VERSION='"{{.Chart.AppVersion}}"' TOP_PATH=".." envsubst < config/templates/labels.yaml.envsubst > config/.helm/labels.yaml
	@VERSION=dummy TOP_PATH=".." envsubst < config/templates/kustomization.yaml.envsubst > config/.helm/kustomization.yaml
	@echo Generate helm crds... >&2
	@$(KUSTOMIZE) build ./config/.helm | $(KUSTOMIZE) cfg grep kind=CustomResourceDefinition | $(SED) -e "1i{{- if .Values.installCRDs }}" -e '$$a{{- end }}' -e '/^  creationTimestamp: null/i \ \ \ \ {{- trim (include "kyverno.crdAnnotations" .) | nindent 4 }}' > ./charts/kyverno/templates/crds.yaml

.PHONY: codegen-helm-all
codegen-helm-all: codegen-helm-crds codegen-helm-docs ## Generate helm docs and CRDs

.PHONY: codegen-install
codegen-install: $(KUSTOMIZE) ## Create install maifests
	@echo Create kustomization... >&2
	@VERSION=latest TOP_PATH="." envsubst < config/templates/labels.yaml.envsubst > config/labels.yaml
	@VERSION=latest TOP_PATH="." envsubst < config/templates/kustomization.yaml.envsubst > config/kustomization.yaml
	@echo Generate install.yaml... >&2
	@$(KUSTOMIZE) build ./config > ./config/install.yaml
	@echo Generate install_debug.yaml... >&2
	@$(KUSTOMIZE) build ./config/debug > ./config/install_debug.yaml

# guidance https://github.com/kyverno/kyverno/wiki/Generate-a-Release
.PHONY: codegen-release
codegen-release: codegen-install $(KUSTOMIZE) ## Create release maifests
	@echo Create release folder... >&2
	@mkdir -p config/.release
	@echo Create kustomization... >&2
	@VERSION=$(GIT_VERSION) TOP_PATH=".." envsubst < config/templates/labels.yaml.envsubst > config/.release/labels.yaml
	@VERSION=$(GIT_VERSION) TOP_PATH=".." envsubst < config/templates/kustomization.yaml.envsubst > config/.release/kustomization.yaml
	@echo Generate release manifests... >&2
	@$(KUSTOMIZE) build ./config/.release > ./config/.release/install.yaml

.PHONY: codegen-quick
codegen-quick: codegen-deepcopy-all codegen-crds-all codegen-api-docs codegen-helm-all codegen-install codegen-release ## Generate all generated code except client

.PHONY: codegen-slow
codegen-slow: codegen-client-all ## Generate client code

.PHONY: codegen-all
codegen-all: codegen-quick codegen-slow ## Generate all generated code

# .PHONY: codegen-openapi
# codegen-openapi: $(PACKAGE_SHIM) $(OPENAPI_GEN) ## Generate open api code
# 	@echo Generate open api definitions... >&2
# 	@GOPATH=$(GOPATH_SHIM) $(OPENAPI_GEN) --go-header-file ./scripts/boilerplate.go.txt \
# 		--input-dirs $(INPUT_DIRS) \
# 		--input-dirs  k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version \
# 		--output-package $(OUT_PACKAGE)/openapi \
# 		-O zz_generated.openapi

##################
# VERIFY CODEGEN #
##################

.PHONY: verify-crds
verify-crds: codegen-crds-all ## Check CRDs are up to date
	@git --no-pager diff config
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-crds-all".' >&2
	@echo 'To correct this, locally run "make codegen-crds-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code config

.PHONY: verify-client
verify-client: codegen-client-all ## Check client is up to date
	@git --no-pager diff --ignore-space-change pkg/client
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-client-all".' >&2
	@echo 'To correct this, locally run "make codegen-client-all", commit the changes, and re-run tests.' >&2
	@git diff --ignore-space-change --quiet --exit-code pkg/client

.PHONY: verify-deepcopy
verify-deepcopy: codegen-deepcopy-all ## Check deepcopy functions are up to date
	@git --no-pager diff api
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-deepcopy-all".' >&2
	@echo 'To correct this, locally run "make codegen-deepcopy-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code api

.PHONY: verify-api-docs
verify-api-docs: codegen-api-docs ## Check api reference docs are up to date
	@git --no-pager diff docs/user
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-api-docs".' >&2
	@echo 'To correct this, locally run "make codegen-api-docs", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code docs/user

.PHONY: verify-helm
verify-helm: codegen-helm-all ## Check Helm charts are up to date
	@git --no-pager diff charts
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-helm-all".' >&2
	@echo 'To correct this, locally run "make codegen-helm", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code charts

.PHONY: verify-codegen
verify-codegen: verify-crds verify-client verify-deepcopy verify-api-docs verify-helm ## Verify all generated code and docs are up to date

##############
# UNIT TESTS #
##############

CODE_COVERAGE_FILE      := coverage
CODE_COVERAGE_FILE_TXT  := $(CODE_COVERAGE_FILE).txt
CODE_COVERAGE_FILE_HTML := $(CODE_COVERAGE_FILE).html

.PHONY: test
test: test-clean test-unit test-e2e ## Clean tests cache then run unit and e2e tests

.PHONY: test-clean
test-clean: ## Clean tests cache
	@echo Clean test cache... >&2
	@go clean -testcache ./...

.PHONY: test-unit
test-unit: test-clean $(GO_ACC) ## Run unit tests
	@echo Running unit tests... >&2
	@$(GO_ACC) ./... -o $(CODE_COVERAGE_FILE_TXT)

.PHONY: code-cov-report
code-cov-report: test-clean ## Generate code coverage report
	@echo Generating code coverage report... >&2
	@GO111MODULE=on go test -v -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out -o $(CODE_COVERAGE_FILE_TXT)
	@go tool cover -html=coverage.out -o $(CODE_COVERAGE_FILE_HTML)

#####################
# CONFORMANCE TESTS #
#####################

.PHONY: test-conformance
test-conformance: ## Run conformance tests
	@echo Running conformance tests... >&2
	@go run ./test/conformance

.PHONY: kind-test-conformance
kind-test-conformance: kind-deploy-kyverno ## Run conformance tests on a local cluster
	@echo Running conformance tests... >&2
	@go run ./test/conformance --create-cluster=false

###############
# KUTTL TESTS #
###############

.PHONY: test-kuttl
test-kuttl: $(KUTTL) ## Run kuttl tests
	@echo Running kuttl tests... >&2
	@$(KUTTL) test --config ./test/conformance/kuttl/kuttl-test.yaml

#############
# CLI TESTS #
#############

TEST_GIT_BRANCH ?= main

.PHONY: test-cli
test-cli: test-cli-policies test-cli-local test-cli-local-mutate test-cli-local-generate test-cli-test-case-selector-flag test-cli-registry ## Run all CLI tests

.PHONY: test-cli-policies
test-cli-policies: $(CLI_BIN)
	@$(CLI_BIN) test https://github.com/kyverno/policies/$(TEST_GIT_BRANCH)

.PHONY: test-cli-local
test-cli-local: $(CLI_BIN)
	@$(CLI_BIN) test ./test/cli/test

.PHONY: test-cli-local-mutate
test-cli-local-mutate: $(CLI_BIN)
	@$(CLI_BIN) test ./test/cli/test-mutate

.PHONY: test-cli-local-generate
test-cli-local-generate: $(CLI_BIN)
	@$(CLI_BIN) test ./test/cli/test-generate

.PHONY: test-cli-test-case-selector-flag
test-cli-test-case-selector-flag: $(CLI_BIN)
	@$(CLI_BIN) test ./test/cli/test --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

.PHONY: test-cli-registry
test-cli-registry: $(CLI_BIN)
	@$(CLI_BIN) test ./test/cli/registry --registry

##################################
# Create e2e Infrastructure
##################################

.PHONY: kind-e2e-cluster
kind-e2e-cluster: $(KIND) ## Create kind cluster for e2e tests
	$(KIND) create cluster --image=$(KIND_IMAGE)

# TODO(eddycharly): $(REPO) is wrong, it is always ghcr.io/kyverno in the source
.PHONY: e2e-kustomize
e2e-kustomize: $(KUSTOMIZE) ## Build kustomize manifests for e2e tests
	cd config && \
	$(KUSTOMIZE) edit set image $(REPO_KYVERNOPRE)=$(LOCAL_KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV) && \
	$(KUSTOMIZE) edit set image $(REPO_KYVERNO)=$(LOCAL_KYVERNO_IMAGE):$(IMAGE_TAG_DEV)
	$(KUSTOMIZE) build config/ -o config/install.yaml

.PHONY: e2e-init-container
e2e-init-container: kind-e2e-cluster | image-build-kyvernopre
	$(KIND) load docker-image $(LOCAL_KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: e2e-kyverno-container
e2e-kyverno-container: kind-e2e-cluster | image-build-kyverno
	$(KIND) load docker-image $(LOCAL_KYVERNO_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: create-e2e-infrastructure
create-e2e-infrastructure: e2e-init-container e2e-kyverno-container e2e-kustomize | ## Setup infrastructure for e2e tests

##################################
# Testing & Code-Coverage
##################################

# Test E2E
test-e2e:
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/verifyimages -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/metrics -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/mutate -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/generate -v

test-e2e-local:
	kubectl apply -f https://raw.githubusercontent.com/kyverno/kyverno/main/config/github/rbac.yaml
	kubectl port-forward -n kyverno service/kyverno-svc-metrics  8000:8000 &
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/verifyimages -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/metrics -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/mutate -v
	E2E=ok K8S_VERSION=$(K8S_VERSION) go test ./test/e2e/generate -v
	kill  $!

helm-test-values:
	sed -i -e "s|nameOverride:.*|nameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|fullnameOverride:.*|fullnameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|namespace:.*|namespace: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|tag:  # replaced in e2e tests.*|tag: $(IMAGE_TAG_DEV)|" charts/kyverno/values.yaml
	sed -i -e "s|repository: ghcr.io/kyverno/kyvernopre  # init: replaced in e2e tests|repository: $(LOCAL_KYVERNOPRE_IMAGE)|" charts/kyverno/values.yaml
	sed -i -e "s|repository: ghcr.io/kyverno/kyverno  # kyverno: replaced in e2e tests|repository: $(LOCAL_KYVERNO_IMAGE)|" charts/kyverno/values.yaml

release-notes:
	@bash -c 'while IFS= read -r line ; do if [[ "$$line" == "## "* && "$$line" != "## $(VERSION)" ]]; then break ; fi; echo "$$line"; done < "CHANGELOG.md"' \
	true

#########
# DEBUG #
#########

.PHONY: debug-deploy
debug-deploy: codegen-install ## Install debug manifests
	@kubectl create -f ./config/install_debug.yaml || kubectl replace -f ./config/install_debug.yaml

##########
# GITHUB #
##########

.PHONY: gh-install-pin-github-action
gh-install-pin-github-action:
	@npm install -g pin-github-action

.PHONY: gh-pin-actions
gh-pin-actions: gh-install-pin-github-action
	@pin-github-action ./.github/workflows/release.yaml

#############
# PERF TEST #
#############

PERF_TEST_NODE_COUNT		?= 3
PERF_TEST_MEMORY_REQUEST	?= "1Gi"

.PHONY: test-perf
test-perf: $(PACKAGE_SHIM)
	GO111MODULE=off GOPATH=$(GOPATH_SHIM) go get k8s.io/perf-tests || true
	cd $(GOPATH_SHIM)/src/k8s.io/perf-tests && \
	GOPATH=$(GOPATH_SHIM) ./run-e2e.sh cluster-loader2 \
		--testconfig=./testing/load/config.yaml \
		--provider=kind \
		--kubeconfig=${HOME}/.kube/config \
		--nodes=$(PERF_TEST_NODE_COUNT) \
		--prometheus-memory-request=$(PERF_TEST_MEMORY_REQUEST) \
		--enable-prometheus-server=true \
		--tear-down-prometheus-server=true \
		--prometheus-apiserver-scrape-port=6443 \
		--prometheus-scrape-kubelets=true \
		--prometheus-scrape-master-kubelets=true \
		--prometheus-scrape-etcd=true \
		--prometheus-scrape-kube-proxy=true \
		--prometheus-kube-proxy-selector-key=k8s-app \
		--prometheus-scrape-node-exporter=false \
		--prometheus-scrape-kube-state-metrics=true \
		--prometheus-scrape-metrics-server=true \
		--prometheus-pvc-storage-class=standard \
		--v=2 \
		--report-dir=.

########
# KIND #
########

.PHONY: kind-create-cluster
kind-create-cluster: $(KIND) ## Create kind cluster
	@echo Create kind cluster... >&2
	@$(KIND) create cluster --name $(KIND_NAME) --image $(KIND_IMAGE) --config ./scripts/kind.yaml

.PHONY: kind-delete-cluster
kind-delete-cluster: $(KIND) ## Delete kind cluster
	@echo Delete kind cluster... >&2
	@$(KIND) delete cluster --name $(KIND_NAME)

.PHONY: kind-load-kyvernopre
kind-load-kyvernopre: $(KIND) image-build-kyvernopre ## Build kyvernopre image and load it in kind cluster
	@echo Load kyvernopre image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: kind-load-kyverno
kind-load-kyverno: $(KIND) image-build-kyverno ## Build kyverno image and load it in kind cluster
	@echo Load kyverno image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_KYVERNO_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: kind-load-all
kind-load-all: kind-load-kyvernopre kind-load-kyverno ## Build images and load them in kind cluster

.PHONY: kind-deploy-kyverno
kind-deploy-kyverno: $(HELM) kind-load-all ## Build images, load them in kind cluster and deploy kyverno helm chart
	@echo Install kyverno chart... >&2
	@$(HELM) upgrade --install kyverno --namespace kyverno --wait --create-namespace ./charts/kyverno \
		--set image.repository=$(LOCAL_KYVERNO_IMAGE) \
		--set image.tag=$(IMAGE_TAG_DEV) \
		--set initImage.repository=$(LOCAL_KYVERNOPRE_IMAGE) \
		--set initImage.tag=$(IMAGE_TAG_DEV) \
		--set initContainer.extraArgs={--loggingFormat=text}
		--set "extraArgs={--autogenInternals=true,--loggingFormat=text}"
	@echo Restart kyverno pods... >&2
	@kubectl rollout restart deployment -n kyverno kyverno

.PHONY: kind-deploy-kyverno-policies
kind-deploy-kyverno-policies: $(HELM) ## Deploy kyverno-policies helm chart
	@echo Install kyverno-policies chart... >&2
	@$(HELM) upgrade --install kyverno-policies --namespace kyverno --wait --create-namespace ./charts/kyverno-policies

.PHONY: kind-deploy-metrics-server
kind-deploy-metrics-server: $(HELM) ## Deploy metrics-server helm chart
	@echo Install metrics-server chart... >&2
	@$(HELM) upgrade --install metrics-server --namespace kube-system --wait --repo https://charts.bitnami.com/bitnami metrics-server \
		--set extraArgs={--kubelet-insecure-tls=true} \
		--set apiService.create=true

.PHONY: kind-deploy-all
kind-deploy-all: kind-deploy-metrics-server | kind-deploy-kyverno kind-deploy-kyverno-policies ## Build images, load them in kind cluster and deploy helm charts

.PHONY: kind-deploy-reporter
kind-deploy-reporter: $(HELM) ## Deploy policy-reporter helm chart
	@echo Install policy-reporter chart... >&2
	@$(HELM) upgrade --install policy-reporter --namespace policy-reporter --wait --repo https://kyverno.github.io/policy-reporter policy-reporter \
		--set ui.enabled=true \
		--set kyvernoPlugin.enabled=true \
		--create-namespace
	@kubectl port-forward -n policy-reporter services/policy-reporter-ui  8082:8080

deploy-kube-prom-stack: $(HELM)
	@$(HELM) upgrade --install kube-prometheus-stack --namespace monitoring --create-namespace --wait \
		--repo https://prometheus-community.github.io/helm-charts kube-prometheus-stack \
		--values ./scripts/kube-prometheus-stack.yaml

########
# HELP #
########

.PHONY: help
help: ## Shows the available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}'
