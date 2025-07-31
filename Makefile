.DEFAULT_GOAL: build-all

############
# DEFAULTS #
############

GIT_SHA              := $(shell git rev-parse HEAD)
REGISTRY             ?= ghcr.io
REPO                 ?= kyverno
KIND_IMAGE           ?= kindest/node:v1.33.1
KIND_NAME            ?= kind
KIND_CONFIG          ?= default
GOOS                 ?= $(shell go env GOOS)
GOARCH               ?= $(shell go env GOARCH)
KOCACHE              ?= /tmp/ko-cache
BUILD_WITH           ?= ko
KYVERNOPRE_IMAGE     := kyvernopre
KYVERNO_IMAGE        := kyverno
CLI_IMAGE            := kyverno-cli
CLEANUP_IMAGE        := cleanup-controller
REPORTS_IMAGE        := reports-controller
BACKGROUND_IMAGE     := background-controller
REPO_KYVERNOPRE      := $(REGISTRY)/$(REPO)/$(KYVERNOPRE_IMAGE)
REPO_KYVERNO         := $(REGISTRY)/$(REPO)/$(KYVERNO_IMAGE)
REPO_CLI             := $(REGISTRY)/$(REPO)/$(CLI_IMAGE)
REPO_CLEANUP         := $(REGISTRY)/$(REPO)/$(CLEANUP_IMAGE)
REPO_REPORTS         := $(REGISTRY)/$(REPO)/$(REPORTS_IMAGE)
REPO_BACKGROUND      := $(REGISTRY)/$(REPO)/$(BACKGROUND_IMAGE)
USE_CONFIG           ?= standard
INSTALL_VERSION	     ?= 3.2.6

#########
# TOOLS #
#########

TOOLS_DIR                          ?= $(PWD)/.tools
KIND                               ?= $(TOOLS_DIR)/kind
KIND_VERSION                       ?= v0.29.0
CONTROLLER_GEN                     := $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION             ?= v0.17.3
CLIENT_GEN                         ?= $(TOOLS_DIR)/client-gen
LISTER_GEN                         ?= $(TOOLS_DIR)/lister-gen
INFORMER_GEN                       ?= $(TOOLS_DIR)/informer-gen
REGISTER_GEN                       ?= $(TOOLS_DIR)/register-gen
DEEPCOPY_GEN                       ?= $(TOOLS_DIR)/deepcopy-gen
CODE_GEN_VERSION                   ?= v0.32.4
GEN_CRD_API_REFERENCE_DOCS         ?= $(TOOLS_DIR)/gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS_VERSION ?= latest
GENREF                             ?= $(TOOLS_DIR)/genref
GENREF_VERSION                     ?= master
GOIMPORTS                          ?= $(TOOLS_DIR)/goimports
GOIMPORTS_VERSION                  ?= latest
HELM                               ?= $(TOOLS_DIR)/helm
HELM_VERSION                       ?= v3.17.3
HELM_DOCS                          ?= $(TOOLS_DIR)/helm-docs
HELM_DOCS_VERSION                  ?= v1.14.2
KO                                 ?= $(TOOLS_DIR)/ko
KO_VERSION                         ?= v0.17.1
API_GROUP_RESOURCES                ?= $(TOOLS_DIR)/api-group-resources
CLIENT_WRAPPER                     ?= $(TOOLS_DIR)/client-wrapper
KUBE_VERSION                       ?= v1.25.0
TOOLS                              := $(KIND) $(CONTROLLER_GEN) $(CLIENT_GEN) $(LISTER_GEN) $(INFORMER_GEN) $(REGISTER_GEN) $(DEEPCOPY_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GENREF) $(GOIMPORTS) $(HELM) $(HELM_DOCS) $(KO) $(CLIENT_WRAPPER)
ifeq ($(GOOS), darwin)
SED                                := gsed
else
SED                                := sed
endif
COMMA                              := ,

$(KIND):
	@echo Install kind... >&2
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kind@$(KIND_VERSION)

$(CONTROLLER_GEN):
	@echo Install controller-gen... >&2
	@cd ./hack/controller-gen && GOBIN=$(TOOLS_DIR) go install -buildvcs=false

$(CLIENT_GEN):
	@echo Install client-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/client-gen@$(CODE_GEN_VERSION)

$(LISTER_GEN):
	@echo Install lister-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/lister-gen@$(CODE_GEN_VERSION)

$(INFORMER_GEN):
	@echo Install informer-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/informer-gen@$(CODE_GEN_VERSION)

$(REGISTER_GEN):
	@echo Install register-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/register-gen@$(CODE_GEN_VERSION)

$(DEEPCOPY_GEN):
	@echo Install deepcopy-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/deepcopy-gen@$(CODE_GEN_VERSION)

$(GEN_CRD_API_REFERENCE_DOCS):
	@echo Install gen-crd-api-reference-docs... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

$(GENREF):
	@echo Install genref... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/kubernetes-sigs/reference-docs/genref@$(GENREF_VERSION)

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

$(API_GROUP_RESOURCES):
	@echo Install api-group-resources... >&2
	@cd ./hack/api-group-resources && GOBIN=$(TOOLS_DIR) go install

$(CLIENT_WRAPPER):
	@echo Install client-wrapper... >&2
	@cd ./hack/client-wrapper && GOBIN=$(TOOLS_DIR) go install

.PHONY: install-tools
install-tools: ## Install tools
install-tools: $(TOOLS)

.PHONY: clean-tools
clean-tools: ## Remove installed tools
	@echo Clean tools... >&2
	@rm -rf $(TOOLS_DIR)

#################
# BUILD (LOCAL) #
#################

CMD_DIR        := cmd
KYVERNO_DIR    := $(CMD_DIR)/kyverno
KYVERNOPRE_DIR := $(CMD_DIR)/kyverno-init
CLI_DIR        := $(CMD_DIR)/cli/kubectl-kyverno
CLEANUP_DIR    := $(CMD_DIR)/cleanup-controller
REPORTS_DIR    := $(CMD_DIR)/reports-controller
BACKGROUND_DIR := $(CMD_DIR)/background-controller
KYVERNO_BIN    := $(KYVERNO_DIR)/kyverno
KYVERNOPRE_BIN := $(KYVERNOPRE_DIR)/kyvernopre
CLI_BIN        := $(CLI_DIR)/kubectl-kyverno
CLEANUP_BIN    := $(CLEANUP_DIR)/cleanup-controller
REPORTS_BIN    := $(REPORTS_DIR)/reports-controller
BACKGROUND_BIN := $(BACKGROUND_DIR)/background-controller
PACKAGE        ?= github.com/kyverno/kyverno
CGO_ENABLED    ?= 0
ifdef VERSION
LD_FLAGS       := "-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(VERSION)"
else
LD_FLAGS       := "-s -w"
endif

.PHONY: fmt
fmt: ## Run go fmt
	@echo Go fmt... >&2
	@go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo Go vet... >&2
	@go vet ./...

.PHONY: imports
imports: $(GOIMPORTS)
	@echo Go imports... >&2
	@$(GOIMPORTS) -w .

.PHONY: fmt-check
fmt-check: fmt
	@echo Checking code format... >&2
	@git --no-pager diff .
	@echo 'If this test fails, it is because the git diff is non-empty after running "make fmt".' >&2
	@echo 'To correct this, locally run "make fmt" and commit the changes.' >&2
	@git diff --quiet --exit-code .

.PHONY: imports-check
imports-check: imports
	@echo Checking go imports... >&2
	@git --no-pager diff .
	@echo 'If this test fails, it is because the git diff is non-empty after running "make imports-check".' >&2
	@echo 'To correct this, locally run "make imports" and commit the changes.' >&2
	@git diff --quiet --exit-code .

.PHONY: unused-package-check
unused-package-check:
	@tidy=$$(go mod tidy); \
	if [ -n "$${tidy}" ]; then \
		echo "go mod tidy checking failed!"; echo "$${tidy}"; echo; \
	fi

$(BACKGROUND_BIN): fmt vet
	@echo Build background controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(BACKGROUND_BIN) -ldflags=$(LD_FLAGS) ./$(BACKGROUND_DIR)

$(CLEANUP_BIN): fmt vet
	@echo Build cleanup controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(CLEANUP_BIN) -ldflags=$(LD_FLAGS) ./$(CLEANUP_DIR)

$(CLI_BIN): fmt vet
	@echo Build cli binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(CLI_BIN) -ldflags=$(LD_FLAGS) ./$(CLI_DIR)

$(KYVERNO_BIN): fmt vet
	@echo Build kyverno binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(KYVERNO_BIN) -ldflags=$(LD_FLAGS) ./$(KYVERNO_DIR)

$(KYVERNOPRE_BIN): fmt vet
	@echo Build kyvernopre binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(KYVERNOPRE_BIN) -ldflags=$(LD_FLAGS) ./$(KYVERNOPRE_DIR)

$(REPORTS_BIN): fmt vet
	@echo Build reports controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o ./$(REPORTS_BIN) -ldflags=$(LD_FLAGS) ./$(REPORTS_DIR)

.PHONY: build-background-controller
build-background-controller: ## Build background controller binary
build-background-controller: $(BACKGROUND_BIN)

.PHONY: build-cleanup-controller
build-cleanup-controller: ## Build cleanup controller binary
build-cleanup-controller: $(CLEANUP_BIN)

.PHONY: build-cli
build-cli: ## Build cli binary
build-cli: $(CLI_BIN)

.PHONY: build-kyverno
build-kyverno: ## Build kyverno binary
build-kyverno: $(KYVERNO_BIN)

.PHONY: build-kyverno-init
build-kyverno-init: ## Build kyvernopre binary
build-kyverno-init: $(KYVERNOPRE_BIN)

.PHONY: build-reports-controller
build-reports-controller: ## Build reports controller binary
build-reports-controller: $(REPORTS_BIN)

build-all: ## Build all binaries
build-all: build-background-controller
build-all: build-cleanup-controller
build-all: build-cli
build-all: build-kyverno
build-all: build-kyverno-init
build-all: build-reports-controller

##############
# BUILD (KO) #
##############

LOCAL_PLATFORM      := linux/$(GOARCH)
KO_REGISTRY         := ko.local
ifndef VERSION
KO_TAGS             := $(GIT_SHA)
else ifeq ($(VERSION),main)
KO_TAGS             := $(GIT_SHA),latest
else
KO_TAGS             := $(GIT_SHA),$(subst /,-,$(VERSION))
endif

KO_CLI_REPO         := $(PACKAGE)/$(CLI_DIR)
KO_KYVERNOPRE_REPO  := $(PACKAGE)/$(KYVERNOPRE_DIR)
KO_KYVERNO_REPO     := $(PACKAGE)/$(KYVERNO_DIR)
KO_CLEANUP_REPO     := $(PACKAGE)/$(CLEANUP_DIR)
KO_REPORTS_REPO     := $(PACKAGE)/$(REPORTS_DIR)
KO_BACKGROUND_REPO  := $(PACKAGE)/$(BACKGROUND_DIR)

.PHONY: ko-build-kyverno-init
ko-build-kyverno-init: $(KO) ## Build kyvernopre local image (with ko)
	@echo Build kyvernopre local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(KYVERNOPRE_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-kyverno
ko-build-kyverno: $(KO) ## Build kyverno local image (with ko)
	@echo Build kyverno local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(KYVERNO_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-cli
ko-build-cli: $(KO) ## Build cli local image (with ko)
	@echo Build cli local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(CLI_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-cleanup-controller
ko-build-cleanup-controller: $(KO) ## Build cleanup controller local image (with ko)
	@echo Build cleanup controller local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(CLEANUP_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-reports-controller
ko-build-reports-controller: $(KO) ## Build reports controller local image (with ko)
	@echo Build reports controller local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(REPORTS_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-background-controller
ko-build-background-controller: $(KO) ## Build background controller local image (with ko)
	@echo Build background controller local image with ko... >&2
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(KO_REGISTRY) \
		$(KO) build ./$(BACKGROUND_DIR) --preserve-import-paths --tags=$(KO_TAGS) --platform=$(LOCAL_PLATFORM)

.PHONY: ko-build-all
ko-build-all: ko-build-kyverno-init ko-build-kyverno ko-build-cli ko-build-cleanup-controller ko-build-reports-controller ko-build-background-controller ## Build all local images (with ko)

################
# PUBLISH (KO) #
################

REGISTRY_USERNAME   ?= dummy
PLATFORMS           := all

.PHONY: ko-login
ko-login: $(KO)
	@$(KO) login $(REGISTRY) --username $(REGISTRY_USERNAME) --password $(REGISTRY_PASSWORD)

.PHONY: ko-publish-kyverno-init
ko-publish-kyverno-init: ko-login ## Build and publish kyvernopre image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNOPRE) \
		$(KO) build ./$(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/kyvernopre'

.PHONY: ko-publish-kyverno
ko-publish-kyverno: ko-login ## Build and publish kyverno image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNO) \
		$(KO) build ./$(KYVERNO_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/kyverno'

.PHONY: ko-publish-cli
ko-publish-cli: ko-login ## Build and publish cli image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLI) \
		$(KO) build ./$(CLI_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno Team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/kyverno-cli'

.PHONY: ko-publish-cleanup-controller
ko-publish-cleanup-controller: ko-login ## Build and publish cleanup controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLEANUP) \
		$(KO) build ./$(CLEANUP_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno Team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/cleanup-controller'

.PHONY: ko-publish-reports-controller
ko-publish-reports-controller: ko-login ## Build and publish reports controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_REPORTS) \
		$(KO) build ./$(REPORTS_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/reports-controller'

.PHONY: ko-publish-background-controller
ko-publish-background-controller: ko-login ## Build and publish background controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_BACKGROUND) \
		$(KO) build ./$(BACKGROUND_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS) \
		--image-annotation 'org.opencontainers.image.authors'='The Kyverno team','org.opencontainers.image.source'='github.com/kyverno/kyverno/commit/${GIT_SHA}','org.opencontainers.image.vendor'='Kyverno','org.opencontainers.image.url'='ghcr.io/kyverno/background-controller'

.PHONY: ko-publish-all
ko-publish-all: ko-publish-kyverno-init ko-publish-kyverno ko-publish-cli ko-publish-cleanup-controller ko-publish-reports-controller ko-publish-background-controller ## Build and publish all images (with ko)

#################
# BUILD (IMAGE) #
#################

LOCAL_REGISTRY         := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_REGISTRY)
LOCAL_CLI_REPO         := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_CLI_REPO)
LOCAL_KYVERNOPRE_REPO  := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_KYVERNOPRE_REPO)
LOCAL_KYVERNO_REPO     := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_KYVERNO_REPO)
LOCAL_CLEANUP_REPO     := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_CLEANUP_REPO)
LOCAL_REPORTS_REPO     := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_REPORTS_REPO)
LOCAL_BACKGROUND_REPO  := $($(shell echo $(BUILD_WITH) | tr '[:lower:]' '[:upper:]')_BACKGROUND_REPO)

.PHONY: image-build-kyverno-init
image-build-kyverno-init: $(BUILD_WITH)-build-kyverno-init

.PHONY: image-build-kyverno
image-build-kyverno: $(BUILD_WITH)-build-kyverno

.PHONY: image-build-cli
image-build-cli: $(BUILD_WITH)-build-cli

.PHONY: image-build-cleanup-controller
image-build-cleanup-controller: $(BUILD_WITH)-build-cleanup-controller

.PHONY: image-build-reports-controller
image-build-reports-controller: $(BUILD_WITH)-build-reports-controller

.PHONY: image-build-background-controller
image-build-background-controller: $(BUILD_WITH)-build-background-controller

.PHONY: image-build-all
image-build-all: $(BUILD_WITH)-build-all

###########
# CODEGEN #
###########

CLIENT_PACKAGE              := $(PACKAGE)/pkg/client
CLIENTSET_PACKAGE           := $(CLIENT_PACKAGE)/clientset
LISTERS_PACKAGE             := $(CLIENT_PACKAGE)/listers
INFORMERS_PACKAGE           := $(CLIENT_PACKAGE)/informers
CRDS_PATH                   := ./config/crds
INSTALL_MANIFEST_PATH       := ./config/install-latest-testing.yaml
KYVERNO_CHART_VERSION       ?= v0.0.0
POLICIES_CHART_VERSION      ?= v0.0.0
APP_CHART_VERSION           ?= latest
KUBE_CHART_VERSION          ?= ">=1.25.0-0"

.PHONY: codegen-api-register
codegen-api-register: ## Generate API types registrations
codegen-api-register: $(REGISTER_GEN)
	@echo Generate registration... >&2
	@$(REGISTER_GEN) --go-header-file=./scripts/boilerplate.go.txt --output-file zz_generated.register.go ./api/...

.PHONY: codegen-api-deepcopy
codegen-api-deepcopy: ## Generate API deep copy functions
codegen-api-deepcopy: $(DEEPCOPY_GEN)
	@echo Generate deep copy functions... >&2
	@$(DEEPCOPY_GEN) --go-header-file ./scripts/boilerplate.go.txt --output-file zz_generated.deepcopy.go ./api/...

.PHONY: codegen-api-docs
codegen-api-docs: ## Generate API docs
codegen-api-docs: $(GEN_CRD_API_REFERENCE_DOCS)
codegen-api-docs: $(GENREF)
	@echo Generate api docs... >&2
	@rm -rf docs/user/crd && mkdir -p docs/user/crd
	@$(GEN_CRD_API_REFERENCE_DOCS) \
		-api-dir $(PACKAGE)/api \
		-config docs/user/config.json \
		-template-dir docs/user/template \
		-out-file docs/user/crd/index.html
	@cd ./docs/user && $(GENREF) \
		-c config-api.yaml \
		-o crd \
		-f html

.PHONY: codegen-api-all
codegen-api-all: ## Generate API related code
codegen-api-all: codegen-api-register
codegen-api-all: codegen-api-deepcopy
codegen-api-all: codegen-api-docs

.PHONY: codegen-client-clientset
codegen-client-clientset: ## Generate clientset
codegen-client-clientset: $(CLIENT_GEN)
	@echo Generate clientset... >&2
	@rm -rf ./pkg/client/clientset && mkdir -p ./pkg/client/clientset
	@$(CLIENT_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--clientset-name versioned \
		--output-dir ./pkg/client/clientset \
		--output-pkg $(CLIENTSET_PACKAGE) \
		--input-base github.com/kyverno/kyverno \
		--input ./api/kyverno/v1 \
		--input ./api/kyverno/v2 \
		--input ./api/kyverno/v2alpha1 \
		--input ./api/reports/v1 \
		--input ./api/policyreport/v1alpha2 \
		--input ./api/policies.kyverno.io/v1alpha1

.PHONY: codegen-client-listers
codegen-client-listers: ## Generate listers
codegen-client-listers: $(LISTER_GEN)
	@echo Generate listers... >&2
	@rm -rf ./pkg/client/listers && mkdir -p ./pkg/client/listers
	@$(LISTER_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--output-dir ./pkg/client/listers \
		--output-pkg $(LISTERS_PACKAGE) \
		./api/kyverno/v1 \
		./api/kyverno/v2 \
		./api/kyverno/v2alpha1 \
		./api/reports/v1 \
		./api/policyreport/v1alpha2 \
		./api/policies.kyverno.io/v1alpha1

.PHONY: codegen-client-informers
codegen-client-informers: ## Generate informers
codegen-client-informers: $(INFORMER_GEN)
	@echo Generate informers... >&2
	@rm -rf ./pkg/client/informers && mkdir -p ./pkg/client/informers
	@$(INFORMER_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--output-dir ./pkg/client/informers \
		--output-pkg $(INFORMERS_PACKAGE) \
		--versioned-clientset-package $(CLIENTSET_PACKAGE)/versioned \
		--listers-package $(LISTERS_PACKAGE) \
		./api/kyverno/v1 \
		./api/kyverno/v2 \
		./api/kyverno/v2alpha1 \
		./api/reports/v1 \
		./api/policyreport/v1alpha2 \
		./api/policies.kyverno.io/v1alpha1

.PHONY: codegen-client-wrappers
codegen-client-wrappers: ## Generate client wrappers
codegen-client-wrappers: codegen-client-clientset
codegen-client-wrappers: $(GOIMPORTS)
codegen-client-wrappers: $(CLIENT_WRAPPER)
	@echo Generate client wrappers... >&2
	@$(CLIENT_WRAPPER)
	@$(GOIMPORTS) -w ./pkg/clients
	@go fmt ./pkg/clients/...

.PHONY: codegen-client-all
codegen-client-all: ## Generate clientset, listers and informers
codegen-client-all: codegen-client-clientset
codegen-client-all: codegen-client-listers
codegen-client-all: codegen-client-informers
codegen-client-all: codegen-client-wrappers

.PHONY: codegen-crds-kyverno
codegen-crds-kyverno: ## Generate kyverno CRDs
codegen-crds-kyverno: $(CONTROLLER_GEN)
	@echo Generate kyverno crds... >&2
	@rm -rf $(CRDS_PATH)/kyverno && mkdir -p $(CRDS_PATH)/kyverno
	@$(CONTROLLER_GEN) \
		paths=./api/kyverno/v1/... \
		paths=./api/kyverno/v1beta1/... \
		paths=./api/kyverno/v2/... \
		paths=./api/kyverno/v2alpha1/... \
		paths=./api/kyverno/v2beta1/... \
		crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false \
		output:dir=$(CRDS_PATH)/kyverno

.PHONY: codegen-crds-policies
codegen-crds-policies: ## Generate policies CRDs
codegen-crds-policies: $(CONTROLLER_GEN)
	@echo Generate policies crds... >&2
	@rm -rf $(CRDS_PATH)/policies.kyverno.io && mkdir -p $(CRDS_PATH)/policies.kyverno.io
	@$(CONTROLLER_GEN) \
		paths=./api/policies.kyverno.io/v1alpha1/... \
		crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false \
		output:dir=$(CRDS_PATH)/policies.kyverno.io

.PHONY: codegen-crds-policyreport
codegen-crds-policyreport: ## Generate policy reports CRDs
codegen-crds-policyreport: $(CONTROLLER_GEN)
	@echo Generate policy reports crds... >&2
	@rm -rf $(CRDS_PATH)/policyreport && mkdir -p $(CRDS_PATH)/policyreport
	@$(CONTROLLER_GEN) \
		paths=./api/policyreport/... \
		crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false \
		output:dir=$(CRDS_PATH)/policyreport

.PHONY: codegen-crds-reports
codegen-crds-reports: ## Generate reports CRDs
codegen-crds-reports: $(CONTROLLER_GEN)
	@echo Generate reports crds... >&2
	@rm -rf $(CRDS_PATH)/reports && mkdir -p $(CRDS_PATH)/reports
	@$(CONTROLLER_GEN) \
		paths=./api/reports/... \
		crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false \
		output:dir=$(CRDS_PATH)/reports

.PHONY: codegen-crds-cli
codegen-crds-cli: ## Generate CLI CRDs
codegen-crds-cli: $(CONTROLLER_GEN)
	@echo Generate cli crds... >&2
	@rm -rf ./cmd/cli/kubectl-kyverno/config/crds && mkdir -p ./cmd/cli/kubectl-kyverno/config/crds
	@$(CONTROLLER_GEN) \
		paths=./cmd/cli/kubectl-kyverno/apis/... \
		crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false \
		output:dir=./cmd/cli/kubectl-kyverno/config/crds

.PHONY: codegen-crds-all
codegen-crds-all: ## Generate all CRDs
codegen-crds-all: codegen-crds-kyverno
codegen-crds-all: codegen-crds-policyreport
codegen-crds-all: codegen-crds-reports
codegen-crds-all: codegen-crds-policies
codegen-crds-all: codegen-crds-cli

.PHONY: codegen-cli-api-group-resources
codegen-cli-api-group-resources: ## Generate API group resources
codegen-cli-api-group-resources: $(API_GROUP_RESOURCES)
codegen-cli-api-group-resources: $(KIND)
	@echo Generate API group resources... >&2
	@$(KIND) delete cluster --name codegen-cli-api-group-resources || true
	@$(KIND) create cluster --name codegen-cli-api-group-resources --image $(KIND_IMAGE) --config ./scripts/config/kind/codegen.yaml
	@$(API_GROUP_RESOURCES) > cmd/cli/kubectl-kyverno/data/api-group-resources.json
	@$(KIND) delete cluster --name codegen-cli-api-group-resources

.PHONY: codegen-cli-crds
codegen-cli-crds: ## Copy generated CRDs to embed in the CLI
codegen-cli-crds: codegen-crds-kyverno
codegen-cli-crds: codegen-crds-policies
codegen-cli-crds: codegen-crds-cli
	@echo Copy generated CRDs to embed in the CLI... >&2
	@rm -rf cmd/cli/kubectl-kyverno/data/crds && mkdir -p cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_clusterpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_policies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_policyexceptions.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_policyexceptions.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_validatingpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_mutatingpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_generatingpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_imagevalidatingpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/policies.kyverno.io/policies.kyverno.io_deletingpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp cmd/cli/kubectl-kyverno/config/crds/* cmd/cli/kubectl-kyverno/data/crds

.PHONY: codegen-cli-docs
codegen-cli-docs: ## Generate CLI docs
codegen-cli-docs: $(CLI_BIN)
	@echo Generate cli docs... >&2
	@rm -rf docs/user/cli/commands && mkdir -p docs/user/cli/commands
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) docs -o docs/user/cli/commands --autogenTag=false

.PHONY: codegen-cli-api-docs
codegen-cli-api-docs: ## Generate CLI API docs
codegen-cli-api-docs: $(GEN_CRD_API_REFERENCE_DOCS)
codegen-cli-api-docs: $(GENREF)
	@echo Generate CLI api docs... >&2
	@rm -rf docs/user/cli/crd && mkdir -p docs/user/cli/crd
	@$(GEN_CRD_API_REFERENCE_DOCS) \
		-api-dir $(PACKAGE)/cmd/cli/kubectl-kyverno/apis \
		-config docs/user/config.json \
		-template-dir docs/user/template \
		-out-file docs/user/cli/crd/index.html
	@cd ./docs/user && $(GENREF) \
		-c config-cli-api.yaml \
		-o cli/crd \
		-f html

.PHONY: codegen-cli-all
codegen-cli-all: ## Generate all CLI related code and docs
codegen-cli-all: codegen-cli-api-group-resources
codegen-cli-all: codegen-cli-crds
codegen-cli-all: codegen-cli-docs
codegen-cli-all: codegen-cli-api-docs

define generate_crd
	@echo "{{- if $(if $(6),and .Values.groups.$(4).$(5) (not .Values.reportsServer.enabled),.Values.groups.$(4).$(5)) }}" > ./charts/kyverno/charts/crds/templates/$(3)/$(1)
	@cat $(CRDS_PATH)/$(2)/$(1) \
		| $(SED) -e '/^  annotations:/a \ \ \ \ {{- end }}' \
 		| $(SED) -e '/^  annotations:/a \ \ \ \ {{- toYaml . | nindent 4 }}' \
		| $(SED) -e '/^  annotations:/a \ \ \ \ {{- with .Values.annotations }}' \
 		| $(SED) -e '/^  annotations:/i \ \ labels:' \
		| $(SED) -e '/^  labels:/a \ \ \ \ {{- include "kyverno.crds.labels" . | nindent 4 }}' \
		| $(SED) -e 's/(devel)/$(CONTROLLER_GEN_VERSION)/' \
 		>> ./charts/kyverno/charts/crds/templates/$(3)/$(1)
	@echo "{{- end }}" >> ./charts/kyverno/charts/crds/templates/$(3)/$(1)
endef

.PHONY: helm-setup-openreports
helm-setup-openreports: $(HELM) ## Add openreports helm repo and build dependencies
	@$(HELM) repo add openreports https://openreports.github.io/reports-api
	@$(HELM) dependency build ./charts/kyverno

.PHONY: helm-dependency-update
helm-dependency-update: $(HELM) ## Update helm dependencies
	@echo Updating helm dependencies... >&2
	@cd charts/kyverno && $(HELM) dependency update

.PHONY: codegen-helm-crds
codegen-helm-crds: ## Generate helm CRDs
codegen-helm-crds: codegen-crds-all
	@echo Generate helm crds... >&2
	@rm -rf ./charts/kyverno/charts/crds/templates/kyverno.io && mkdir -p ./charts/kyverno/charts/crds/templates/kyverno.io
	@rm -rf ./charts/kyverno/charts/crds/templates/reports.kyverno.io && mkdir -p ./charts/kyverno/charts/crds/templates/reports.kyverno.io
	@rm -rf ./charts/kyverno/charts/crds/templates/wgpolicyk8s.io && mkdir -p ./charts/kyverno/charts/crds/templates/wgpolicyk8s.io
	@rm -rf ./charts/kyverno/charts/crds/templates/policies.kyverno.io && mkdir -p ./charts/kyverno/charts/crds/templates/policies.kyverno.io
	$(call generate_crd,kyverno.io_cleanuppolicies.yaml,kyverno,kyverno.io,kyverno,cleanuppolicies)
	$(call generate_crd,kyverno.io_clustercleanuppolicies.yaml,kyverno,kyverno.io,kyverno,clustercleanuppolicies)
	$(call generate_crd,kyverno.io_clusterpolicies.yaml,kyverno,kyverno.io,kyverno,clusterpolicies)
	$(call generate_crd,kyverno.io_globalcontextentries.yaml,kyverno,kyverno.io,kyverno,globalcontextentries)
	$(call generate_crd,kyverno.io_policies.yaml,kyverno,kyverno.io,kyverno,policies)
	$(call generate_crd,kyverno.io_policyexceptions.yaml,kyverno,kyverno.io,kyverno,policyexceptions)
	$(call generate_crd,kyverno.io_updaterequests.yaml,kyverno,kyverno.io,kyverno,updaterequests)
	$(call generate_crd,policies.kyverno.io_policyexceptions.yaml,policies.kyverno.io,policies.kyverno.io,policies,policyexceptions)
	$(call generate_crd,policies.kyverno.io_validatingpolicies.yaml,policies.kyverno.io,policies.kyverno.io,policies,validatingpolicies)
	$(call generate_crd,policies.kyverno.io_imagevalidatingpolicies.yaml,policies.kyverno.io,policies.kyverno.io,policies,imagevalidatingpolicies)
	$(call generate_crd,policies.kyverno.io_generatingpolicies.yaml,policies.kyverno.io,policies.kyverno.io,policies,generatingpolicies)
	$(call generate_crd,policies.kyverno.io_mutatingpolicies.yaml,policies.kyverno.io,policies.kyverno.io,policies,mutatingpolicies)
	$(call generate_crd,policies.kyverno.io_deletingpolicies.yaml,policies.kyverno.io,policies.kyverno.io,policies,deletingpolicies)
	$(call generate_crd,reports.kyverno.io_clusterephemeralreports.yaml,reports,reports.kyverno.io,reports,clusterephemeralreports,true)
	$(call generate_crd,reports.kyverno.io_ephemeralreports.yaml,reports,reports.kyverno.io,reports,ephemeralreports,true)
	$(call generate_crd,wgpolicyk8s.io_clusterpolicyreports.yaml,policyreport,wgpolicyk8s.io,wgpolicyk8s,clusterpolicyreports,true)
	$(call generate_crd,wgpolicyk8s.io_policyreports.yaml,policyreport,wgpolicyk8s.io,wgpolicyk8s,policyreports,true)

.PHONY: codegen-helm-docs
codegen-helm-docs: ## Generate helm docs
	@echo Generate helm docs... >&2
	@docker run -v ${PWD}/charts:/work -w /work jnorwood/helm-docs:$(HELM_DOCS_VERSION)

.PHONY: codegen-helm-all
codegen-helm-all: ## Generate helm docs and CRDs
codegen-helm-all: helm-setup-openreports
codegen-helm-all: codegen-helm-crds
codegen-helm-all: codegen-helm-docs

.PHONY: codegen-manifest-install-latest
codegen-manifest-install-latest: ## Create install_latest manifest
codegen-manifest-install-latest:
	@echo Generate latest install manifest... >&2
	@rm -f $(INSTALL_MANIFEST_PATH)
	@$(HELM) template kyverno --kube-version $(KUBE_VERSION) --namespace kyverno --skip-tests ./charts/kyverno \
		--set templating.enabled=true \
		--set templating.version=latest \
		--set admissionController.container.image.tag=latest \
		--set admissionController.initContainer.image.tag=latest \
		--set cleanupController.image.tag=latest \
		--set reportsController.image.tag=latest \
		--set backgroundController.image.tag=latest \
 		| $(SED) -e '/^#.*/d' \
		> $(INSTALL_MANIFEST_PATH)

.PHONY: codegen-manifest-debug
codegen-manifest-debug: ## Create debug manifest
codegen-manifest-debug: helm-setup-openreports
	@echo Generate debug manifest... >&2
	@mkdir -p ./.manifest
	@$(HELM) template kyverno --kube-version $(KUBE_VERSION) --namespace kyverno --skip-tests ./charts/kyverno \
		--set templating.enabled=true \
		--set templating.version=latest \
		--set templating.debug=true \
		--set admissionController.container.image.tag=latest \
		--set admissionController.initContainer.image.tag=latest \
		--set cleanupController.image.tag=latest \
		--set reportsController.image.tag=latest \
 		| $(SED) -e '/^#.*/d' \
		> ./.manifest/debug.yaml

.PHONY: codegen-manifest-release
codegen-manifest-release: ## Create release manifest
codegen-manifest-release: helm-setup-openreportss
	@echo Generate release manifest... >&2
	@mkdir -p ./.manifest
	@$(HELM) template kyverno --kube-version $(KUBE_VERSION) --namespace kyverno --skip-tests ./charts/kyverno \
		--set templating.enabled=true \
		--set templating.version=$(VERSION) \
		--set admissionController.container.image.tag=$(VERSION) \
		--set admissionController.initContainer.image.tag=$(VERSION) \
		--set cleanupController.image.tag=$(VERSION) \
		--set reportsController.image.tag=$(VERSION) \
 		| $(SED) -e '/^#.*/d' \
		> ./.manifest/release.yaml

.PHONY: codegen-manifest-all
codegen-manifest-all: ## Create all manifests
codegen-manifest-all: codegen-manifest-install-latest
codegen-manifest-all: codegen-manifest-debug

.PHONY: codegen-fix-tests
codegen-fix-tests: $(CLI_BIN) ## Fix CLI test files
	@echo Fix CLI test files... >&2
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) fix test test --save --compress --force

.PHONY: codegen-fix-policies
codegen-fix-policies: $(CLI_BIN) ## Fix CLI policy files
	@echo Fix CLI policy files... >&2
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) fix policy . --save

.PHONY: codegen-fix-all
codegen-fix-all: ## Fixes files
codegen-fix-all: codegen-fix-tests
# TODO: fix this target
# codegen-fix-all: codegen-fix-policies

.PHONY: codegen-all
codegen-all: ## Generate all generated code
codegen-all: codegen-api-all
codegen-all: codegen-client-all
codegen-all: codegen-crds-all
codegen-all: codegen-cli-all
codegen-all: codegen-helm-all
codegen-all: codegen-manifest-all
codegen-all: codegen-fix-all

.PHONY: codegen-helm-update-versions
codegen-helm-update-versions: ## Update helm charts versions
	@echo Updating Chart.yaml files... >&2
	@$(SED) -i 's/version: .*/version: $(POLICIES_CHART_VERSION)/' 		charts/kyverno-policies/Chart.yaml
	@$(SED) -i 's/appVersion: .*/appVersion: $(APP_CHART_VERSION)/' 	charts/kyverno-policies/Chart.yaml
	@$(SED) -i 's/kubeVersion: .*/kubeVersion: $(KUBE_CHART_VERSION)/' 	charts/kyverno-policies/Chart.yaml
	@$(SED) -i 's/version: .*/version: $(KYVERNO_CHART_VERSION)/' 		charts/kyverno/Chart.yaml
	@$(SED) -i 's/appVersion: .*/appVersion: $(APP_CHART_VERSION)/' 	charts/kyverno/Chart.yaml
	@$(SED) -i 's/kubeVersion: .*/kubeVersion: $(KUBE_CHART_VERSION)/' 	charts/kyverno/Chart.yaml
	@$(SED) -i 's/version: .*/version: $(KYVERNO_CHART_VERSION)/' 		charts/kyverno/charts/crds/Chart.yaml
	@$(SED) -i 's/appVersion: .*/appVersion: $(APP_CHART_VERSION)/' 	charts/kyverno/charts/crds/Chart.yaml
	@$(SED) -i 's/kubeVersion: .*/kubeVersion: $(KUBE_CHART_VERSION)/' 	charts/kyverno/charts/crds/Chart.yaml
	@$(SED) -i 's/version: .*/version: $(KYVERNO_CHART_VERSION)/' 		charts/kyverno/charts/grafana/Chart.yaml
	@$(SED) -i 's/appVersion: .*/appVersion: $(APP_CHART_VERSION)/' 	charts/kyverno/charts/grafana/Chart.yaml
	@$(SED) -i 's/kubeVersion: .*/kubeVersion: $(KUBE_CHART_VERSION)/' 	charts/kyverno/charts/grafana/Chart.yaml

##################
# VERIFY CODEGEN #
##################

.PHONY: verify-codegen
verify-codegen: ## Verify all generated code and docs are up to date
verify-codegen: codegen-all
	@echo Checking git diff... >&2
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-fix-tests".' >&2
	@echo 'To correct this, locally run "make codegen-fix-tests", commit the changes, and re-run tests.' >&2
	@git diff --exit-code

##############
# UNIT TESTS #
##############

CODE_COVERAGE_FILE      := coverage
CODE_COVERAGE_FILE_OUT  := $(CODE_COVERAGE_FILE).out

.PHONY: test-clean
test-clean: ## Clean tests cache
	@echo Clean test cache... >&2
	@go clean -testcache

.PHONY: test-unit
test-unit: ## Run unit tests
test-unit: test-clean
	@echo Running unit tests... >&2
	@go test -race -covermode atomic -coverprofile $(CODE_COVERAGE_FILE_OUT) ./...

#############
# CLI TESTS #
#############

TEST_GIT_BRANCH ?= main
TEST_GIT_REPO   ?= https://github.com/kyverno/policies

.PHONY: test-cli
test-cli: test-cli-policies test-cli-local ## Run all CLI tests

.PHONY: test-cli-policies
test-cli-policies: $(CLI_BIN) ## Run CLI tests against the policies repository
	@echo Running cli tests against $(TEST_GIT_REPO)/$(TEST_GIT_BRANCH)... >&2
	@$(CLI_BIN) test $(TEST_GIT_REPO)/$(TEST_GIT_BRANCH)

.PHONY: test-cli-local
test-cli-local: test-cli-local-validate test-cli-local-vpols test-cli-local-gpols test-cli-local-mpols test-cli-local-ivpols test-cli-local-dpols test-cli-local-vaps test-cli-local-maps test-cli-local-mutate test-cli-local-generate test-cli-local-exceptions test-cli-local-cel-exceptions test-cli-local-registry test-cli-local-scenarios test-cli-local-selector ## Run local CLI tests

.PHONY: test-cli-local-validate
test-cli-local-validate: $(CLI_BIN) ## Run local CLI validation tests
	@echo Running local cli validation tests... >&2
	@$(CLI_BIN) test ./test/cli/test

.PHONY: test-cli-local-vpols
test-cli-local-vpols: $(CLI_BIN) ## Run local CLI VPOL tests
	@echo Running local cli vpol tests... >&2
	@$(CLI_BIN) test ./test/cli/test-validating-policy

.PHONY: test-cli-local-gpols
test-cli-local-gpols: $(CLI_BIN) ## Run local CLI GPOL tests
	@echo Running local cli gpol tests... >&2
	@$(CLI_BIN) test ./test/cli/test-generating-policy

.PHONY: test-cli-local-mpols
test-cli-local-mpols: $(CLI_BIN) ## Run local CLI GPOL tests
	@echo Running local cli mpol tests... >&2
	@$(CLI_BIN) test ./test/cli/test-mutating-policy

.PHONY: test-cli-local-ivpols
test-cli-local-ivpols: $(CLI_BIN) ## Run local CLI IVPOL tests
	@echo Running local cli ivpol tests... >&2
	@$(CLI_BIN) test ./test/cli/test-image-validating-policy

.PHONY: test-cli-local-dpols
test-cli-local-dpols: $(CLI_BIN) ## Run local CLI IVPOL tests
	@echo Running local cli dpols tests... >&2
	@$(CLI_BIN) test ./test/cli/test-deleting-policy

.PHONY: test-cli-local-vaps
test-cli-local-vaps: $(CLI_BIN) ## Run local CLI VAP tests
	@echo Running local cli vap tests... >&2
	@$(CLI_BIN) test ./test/cli/test-validating-admission-policy

.PHONY: test-cli-local-maps
test-cli-local-maps: $(CLI_BIN) ## Run local CLI MAP tests
	@echo Running local cli MAP tests... >&2
	@$(CLI_BIN) test ./test/cli/test-mutating-admission-policy

.PHONY: test-cli-local-mutate
test-cli-local-mutate: $(CLI_BIN) ## Run local CLI mutation tests
	@echo Running local cli mutation tests... >&2
	@$(CLI_BIN) test ./test/cli/test-mutate

.PHONY: test-cli-local-generate
test-cli-local-generate: $(CLI_BIN) ## Run local CLI generation tests
	@echo Running local cli generation tests... >&2
	@$(CLI_BIN) test ./test/cli/test-generate

.PHONY: test-cli-local-exceptions
test-cli-local-exceptions: $(CLI_BIN) ## Run local CLI exception tests
	@echo Running local cli exception tests... >&2
	@$(CLI_BIN) test ./test/cli/test-exceptions

.PHONY: test-cli-local-cel-exceptions
test-cli-local-cel-exceptions: $(CLI_BIN) ## Run local CLI cel exception tests
	@echo Running local cli cel exception tests... >&2
	@$(CLI_BIN) test ./test/cli/test-cel-exceptions

.PHONY: test-cli-local-selector
test-cli-local-selector: $(CLI_BIN) ## Run local CLI tests (with test case selector)
	@echo Running local cli selector tests... >&2
	@$(CLI_BIN) test ./test/cli/test --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

.PHONY: test-cli-local-registry
test-cli-local-registry: $(CLI_BIN) ## Run local CLI registry tests
	@echo Running local cli registry tests... >&2
	@$(CLI_BIN) test ./test/cli/registry --registry

.PHONY: test-cli-local-scenarios
test-cli-local-scenarios: $(CLI_BIN) ## Run local CLI scenarios tests
	@echo Running local cli scenarios tests... >&2
	@$(CLI_BIN) test ./test/cli/scenarios_to_cli --registry

#############
# HELM TEST #
#############

.PHONY: helm-test
helm-test: $(HELM) ## Run helm test
	@echo Running helm test... >&2
	@$(HELM) dependency build ./charts/kyverno
	@$(HELM) test --namespace kyverno kyverno

#################
# RELEASE NOTES #
#################

.PHONY: release-notes
release-notes: ## Generate release notes
	@echo Generating release notes... >&2
	@bash -c 'while IFS= read -r line ; do if [[ "$$line" == "## "* && "$$line" != "## $(VERSION)" ]]; then break ; fi; echo "$$line"; done < "CHANGELOG.md"' \
	true

#########
# DEBUG #
#########

.PHONY: debug-deploy
debug-deploy: codegen-manifest-debug ## Install debug manifests
	@kubectl create -f ./.manifest/debug.yaml || kubectl replace -f ./.manifest/debug.yaml

##########
# DOCKER #
##########

.PHONY: docker-save-image-all
docker-save-image-all: $(KIND) image-build-all ## Save docker images in archive
	docker save \
		$(LOCAL_REGISTRY)/$(LOCAL_KYVERNOPRE_REPO):$(GIT_SHA) \
		$(LOCAL_REGISTRY)/$(LOCAL_KYVERNO_REPO):$(GIT_SHA) \
		$(LOCAL_REGISTRY)/$(LOCAL_CLEANUP_REPO):$(GIT_SHA) \
		$(LOCAL_REGISTRY)/$(LOCAL_REPORTS_REPO):$(GIT_SHA) \
		$(LOCAL_REGISTRY)/$(LOCAL_BACKGROUND_REPO):$(GIT_SHA) \
		$(LOCAL_REGISTRY)/$(LOCAL_CLI_REPO):$(GIT_SHA) \
	> kyverno.tar

########
# KIND #
########

.PHONY: kind-create-cluster
kind-create-cluster: $(KIND) ## Create kind cluster
	@echo Create kind cluster... >&2
	@$(KIND) create cluster --name $(KIND_NAME) --image $(KIND_IMAGE) --config ./scripts/config/kind/$(KIND_CONFIG).yaml

.PHONY: kind-delete-cluster
kind-delete-cluster: $(KIND) ## Delete kind cluster
	@echo Delete kind cluster... >&2
	@$(KIND) delete cluster --name $(KIND_NAME)

.PHONY: kind-load-kyverno-init
kind-load-kyverno-init: $(KIND) image-build-kyverno-init ## Build kyvernopre image and load it in kind cluster
	@echo Load kyvernopre image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_KYVERNOPRE_REPO):$(GIT_SHA)

.PHONY: kind-load-cli
kind-load-cli: $(KIND) image-build-cli ## Build cli image and load it in kind cluster
	@echo Load cli image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_CLI_REPO):$(GIT_SHA)

.PHONY: kind-load-kyverno
kind-load-kyverno: $(KIND) image-build-kyverno ## Build kyverno image and load it in kind cluster
	@echo Load kyverno image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_KYVERNO_REPO):$(GIT_SHA)

.PHONY: kind-load-cleanup-controller
kind-load-cleanup-controller: $(KIND) image-build-cleanup-controller ## Build cleanup controller image and load it in kind cluster
	@echo Load cleanup controller image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_CLEANUP_REPO):$(GIT_SHA)

.PHONY: kind-load-reports-controller
kind-load-reports-controller: $(KIND) image-build-reports-controller ## Build reports controller image and load it in kind cluster
	@echo Load reports controller image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_REPORTS_REPO):$(GIT_SHA)

.PHONY: kind-load-background-controller
kind-load-background-controller: $(KIND) image-build-background-controller ## Build background controller image and load it in kind cluster
	@echo Load background controller image... >&2
	@$(KIND) load docker-image --name $(KIND_NAME) $(LOCAL_REGISTRY)/$(LOCAL_BACKGROUND_REPO):$(GIT_SHA)

.PHONY: kind-load-all
kind-load-all: ## Build images and load them in kind cluster
kind-load-all: kind-load-kyverno-init
kind-load-all: kind-load-kyverno
kind-load-all: kind-load-cleanup-controller
kind-load-all: kind-load-reports-controller
kind-load-all: kind-load-background-controller
kind-load-all: kind-load-cli

.PHONY: kind-load-image-archive
kind-load-image-archive: $(KIND) ## Load docker images from archive
	@echo Load image archive in kind cluster... >&2
	@$(KIND) load image-archive kyverno.tar --name $(KIND_NAME)

.PHONY: kind-install-kyverno
kind-install-kyverno: helm-setup-openreports ## Install kyverno helm chart
	@echo Install kyverno chart... >&2
	@$(HELM) upgrade --install kyverno --namespace kyverno --create-namespace --wait ./charts/kyverno \
		--set admissionController.container.image.registry=$(LOCAL_REGISTRY) \
		--set admissionController.container.image.repository=$(LOCAL_KYVERNO_REPO) \
		--set admissionController.container.image.tag=$(GIT_SHA) \
		--set admissionController.initContainer.image.registry=$(LOCAL_REGISTRY) \
		--set admissionController.initContainer.image.repository=$(LOCAL_KYVERNOPRE_REPO) \
		--set admissionController.initContainer.image.tag=$(GIT_SHA) \
		--set cleanupController.image.registry=$(LOCAL_REGISTRY) \
		--set cleanupController.image.repository=$(LOCAL_CLEANUP_REPO) \
		--set cleanupController.image.tag=$(GIT_SHA) \
		--set reportsController.image.registry=$(LOCAL_REGISTRY) \
		--set reportsController.image.repository=$(LOCAL_REPORTS_REPO) \
		--set reportsController.image.tag=$(GIT_SHA) \
		--set backgroundController.image.registry=$(LOCAL_REGISTRY) \
		--set backgroundController.image.repository=$(LOCAL_BACKGROUND_REPO) \
		--set backgroundController.image.tag=$(GIT_SHA) \
		--set crds.migration.image.registry=$(LOCAL_REGISTRY) \
		--set crds.migration.image.repository=$(LOCAL_CLI_REPO) \
		--set crds.migration.image.tag=$(GIT_SHA) \
		--values ./scripts/config/resources/kyverno.yaml \
		$(foreach CONFIG,$(subst $(COMMA), ,$(USE_CONFIG)),--values ./scripts/config/$(CONFIG)/kyverno.yaml) \
		$(EXPLICIT_INSTALL_SETTINGS)

.PHONY: kind-install-kyverno-from-repo
kind-install-kyverno-from-repo: $(HELM) ## Install Kyverno Helm Chart from the Kyverno repo
	@echo Install kyverno chart... >&2
	@$(HELM) upgrade --install kyverno --namespace kyverno --create-namespace --wait \
		--repo https://kyverno.github.io/kyverno/ kyverno \
		--version $(INSTALL_VERSION) \
		$(foreach CONFIG,$(subst $(COMMA), ,$(USE_CONFIG)),--values ./scripts/config/$(CONFIG)/kyverno.yaml) \
		$(EXPLICIT_INSTALL_SETTINGS)

.PHONY: kind-install-goldilocks
kind-install-goldilocks: $(HELM) ## Install goldilocks helm chart
	@echo Install goldilocks chart... >&2
	@$(HELM) upgrade --install vpa --namespace vpa --create-namespace --wait \
		--repo https://charts.fairwinds.com/stable vpa
	@$(HELM) upgrade --install goldilocks --namespace goldilocks --create-namespace --wait \
		--repo https://charts.fairwinds.com/stable goldilocks
	kubectl label ns kyverno goldilocks.fairwinds.com/enabled=true

.PHONY: kind-deploy-kyverno
kind-deploy-kyverno: $(HELM) kind-load-all ## Build images, load them in kind cluster and deploy kyverno helm chart
	@$(MAKE) kind-install-kyverno

.PHONY: kind-deploy-kyverno-policies
kind-deploy-kyverno-policies: $(HELM) ## Deploy kyverno-policies helm chart
	@echo Install kyverno-policies chart... >&2
	@$(HELM) upgrade --install kyverno-policies --namespace kyverno --create-namespace --wait ./charts/kyverno-policies \
		$(foreach CONFIG,$(subst $(COMMA), ,$(USE_CONFIG)),--values ./scripts/config/$(CONFIG)/kyverno-policies.yaml)

.PHONY: kind-deploy-all
kind-deploy-all: | kind-deploy-kyverno kind-deploy-kyverno-policies ## Build images, load them in kind cluster and deploy helm charts

.PHONY: kind-deploy-reporter
kind-deploy-reporter: $(HELM) ## Deploy policy-reporter helm chart
	@echo Install policy-reporter chart... >&2
	@$(HELM) upgrade --install policy-reporter --namespace policy-reporter --create-namespace --wait \
		--repo https://kyverno.github.io/policy-reporter policy-reporter \
		--values ./scripts/config/standard/kyverno-reporter.yaml
	@kubectl port-forward -n policy-reporter services/policy-reporter-ui  8082:8080

.PHONY: kind-admission-controller-image-name
kind-admission-controller-image-name: ## Print admission controller image name
	@echo -n $(LOCAL_REGISTRY)/$(LOCAL_KYVERNO_REPO):$(GIT_SHA)

###########
# ROLLOUT #
###########

.PHONY: rollout-cleanup-controller
rollout-cleanup-controller: ## Rollout cleanup-controller deployment
	@kubectl rollout restart deployment -n kyverno -l app.kubernetes.io/component=cleanup-controller

.PHONY: rollout-reports-controller
rollout-reports-controller: ## Rollout reports-controller deployment
	@kubectl rollout restart deployment -n kyverno -l app.kubernetes.io/component=reports-controller

.PHONY: rollout-admission-controller
rollout-admission-controller: ## Rollout admission-controller deployment
	@kubectl rollout restart deployment -n kyverno -l app.kubernetes.io/component=admission-controller

.PHONY: rollout-all
rollout-all: rollout-cleanup-controller rollout-reports-controller rollout-admission-controller ## Rollout all deployment

###########
# DEV LAB #
###########

.PHONY: dev-lab-ingress-ngingx
dev-lab-ingress-ngingx: ## Deploy ingress-ngingx
	@echo Install ingress-ngingx... >&2
	@kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	@sleep 15
	@kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s

.PHONY: dev-lab-prometheus
dev-lab-prometheus: $(HELM) ## Deploy kube-prometheus-stack helm chart
	@echo Install kube-prometheus-stack chart... >&2
	@$(HELM) upgrade --install kube-prometheus-stack --namespace monitoring --create-namespace --wait \
		--repo https://prometheus-community.github.io/helm-charts kube-prometheus-stack \
		--values ./scripts/config/dev/kube-prometheus-stack.yaml

.PHONY: dev-lab-loki
dev-lab-loki: $(HELM) ## Deploy loki-stack helm chart
	@echo Install loki-stack chart... >&2
	@$(HELM) upgrade --install loki-stack --namespace monitoring --create-namespace --wait \
		--repo https://grafana.github.io/helm-charts loki-stack \
		--values ./scripts/config/dev/loki-stack.yaml

.PHONY: dev-lab-tempo
dev-lab-tempo: $(HELM) ## Deploy tempo helm chart
	@echo Install tempo chart... >&2
	@$(HELM) upgrade --install tempo --namespace monitoring --create-namespace --wait \
		--repo https://grafana.github.io/helm-charts tempo \
		--values ./scripts/config/dev/tempo.yaml
	@kubectl apply -f ./scripts/config/dev/tempo-datasource.yaml

.PHONY: dev-lab-otel-collector
dev-lab-otel-collector: $(HELM) ## Deploy tempo helm chart
	@echo Install otel-collector chart... >&2
	@$(HELM) upgrade --install opentelemetry-collector --namespace monitoring --create-namespace --wait \
		--repo https://open-telemetry.github.io/opentelemetry-helm-charts opentelemetry-collector \
		--values ./scripts/config/dev/otel-collector.yaml

.PHONY: dev-lab-metrics-server
dev-lab-metrics-server: $(HELM) ## Deploy metrics-server helm chart
	@echo Install metrics-server chart... >&2
	@$(HELM) install metrics-server oci://registry-1.docker.io/bitnamicharts/metrics-server \
		--namespace kube-system --wait \
		--values ./scripts/config/dev/metrics-server.yaml

.PHONY: dev-lab-all
dev-lab-all: dev-lab-ingress-ngingx dev-lab-metrics-server dev-lab-prometheus dev-lab-loki dev-lab-tempo dev-lab-otel-collector ## Deploy all dev lab components

.PHONY: dev-lab-policy-reporter
dev-lab-policy-reporter: $(HELM) ## Deploy policy-reporter helm chart
	@echo Install policy-reporter chart... >&2
	@$(HELM) upgrade --install policy-reporter --namespace policy-reporter --create-namespace --wait \
		--repo https://kyverno.github.io/policy-reporter policy-reporter \
		--values ./scripts/config/dev/policy-reporter.yaml

.PHONY: dev-lab-kwok
dev-lab-kwok: ## Deploy kwok
	@kubectl apply -k ./scripts/config/kwok

########
# HELP #
########

.PHONY: help
help: ## Shows the available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-40s\033[0m %s\n", $$1, $$2}'
