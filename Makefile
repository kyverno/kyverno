.DEFAULT_GOAL: build-all

############
# DEFAULTS #
############

GIT_SHA              := $(shell git rev-parse HEAD)
REGISTRY             ?= ghcr.io
REPO                 ?= kyverno
KIND_IMAGE           ?= kindest/node:v1.30.0
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
KIND_VERSION                       ?= v0.23.0
CONTROLLER_GEN                     := $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION             ?= v0.16.1
CLIENT_GEN                         ?= $(TOOLS_DIR)/client-gen
LISTER_GEN                         ?= $(TOOLS_DIR)/lister-gen
INFORMER_GEN                       ?= $(TOOLS_DIR)/informer-gen
OPENAPI_GEN                        ?= $(TOOLS_DIR)/openapi-gen
REGISTER_GEN                       ?= $(TOOLS_DIR)/register-gen
DEEPCOPY_GEN                       ?= $(TOOLS_DIR)/deepcopy-gen
DEFAULTER_GEN                      ?= $(TOOLS_DIR)/defaulter-gen
APPLYCONFIGURATION_GEN             ?= $(TOOLS_DIR)/applyconfiguration-gen
CODE_GEN_VERSION                   ?= v0.28.0
GEN_CRD_API_REFERENCE_DOCS         ?= $(TOOLS_DIR)/gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS_VERSION ?= latest
GENREF                             ?= $(TOOLS_DIR)/genref
GENREF_VERSION                     ?= master
GO_ACC                             ?= $(TOOLS_DIR)/go-acc
GO_ACC_VERSION                     ?= latest
GOIMPORTS                          ?= $(TOOLS_DIR)/goimports
GOIMPORTS_VERSION                  ?= latest
HELM                               ?= $(TOOLS_DIR)/helm
HELM_VERSION                       ?= v3.12.3
HELM_DOCS                          ?= $(TOOLS_DIR)/helm-docs
HELM_DOCS_VERSION                  ?= v1.11.0
KO                                 ?= $(TOOLS_DIR)/ko
KO_VERSION                         ?= v0.14.1
KUBE_VERSION                       ?= v1.25.0
TOOLS                              := $(KIND) $(CONTROLLER_GEN) $(CLIENT_GEN) $(LISTER_GEN) $(INFORMER_GEN) $(OPENAPI_GEN) $(REGISTER_GEN) $(DEEPCOPY_GEN) $(DEFAULTER_GEN) $(APPLYCONFIGURATION_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GENREF) $(GO_ACC) $(GOIMPORTS) $(HELM) $(HELM_DOCS) $(KO)
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
	@cd ./hack/controller-gen && GOBIN=$(TOOLS_DIR) go install

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

$(REGISTER_GEN):
	@echo Install register-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/register-gen@$(CODE_GEN_VERSION)

$(DEEPCOPY_GEN):
	@echo Install deepcopy-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/deepcopy-gen@$(CODE_GEN_VERSION)

$(DEFAULTER_GEN):
	@echo Install defaulter-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/defaulter-gen@$(CODE_GEN_VERSION)

$(APPLYCONFIGURATION_GEN):
	@echo Install applyconfiguration-gen... >&2
	@GOBIN=$(TOOLS_DIR) go install k8s.io/code-generator/cmd/applyconfiguration-gen@$(CODE_GEN_VERSION)

$(GEN_CRD_API_REFERENCE_DOCS):
	@echo Install gen-crd-api-reference-docs... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

$(GENREF):
	@echo Install genref... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/kubernetes-sigs/reference-docs/genref@$(GENREF_VERSION)

$(GO_ACC):
	@echo Install go-acc... >&2
	@GOBIN=$(TOOLS_DIR) go install github.com/ory/go-acc@$(GO_ACC_VERSION)

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

.PHONY: install-tools
install-tools: $(TOOLS) ## Install tools

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

$(KYVERNOPRE_BIN): fmt vet
	@echo Build kyvernopre binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(KYVERNOPRE_BIN) -ldflags=$(LD_FLAGS) ./$(KYVERNOPRE_DIR)

$(KYVERNO_BIN): fmt vet
	@echo Build kyverno binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(KYVERNO_BIN) -ldflags=$(LD_FLAGS) ./$(KYVERNO_DIR)

$(CLI_BIN): fmt vet
	@echo Build cli binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(CLI_BIN) -ldflags=$(LD_FLAGS) ./$(CLI_DIR)

$(CLEANUP_BIN): fmt vet
	@echo Build cleanup controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(CLEANUP_BIN) -ldflags=$(LD_FLAGS) ./$(CLEANUP_DIR)

$(REPORTS_BIN): fmt vet
	@echo Build reports controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(REPORTS_BIN) -ldflags=$(LD_FLAGS) ./$(REPORTS_DIR)

$(BACKGROUND_BIN): fmt vet
	@echo Build background controller binary... >&2
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) \
		go build -o ./$(BACKGROUND_BIN) -ldflags=$(LD_FLAGS) ./$(BACKGROUND_DIR)

.PHONY: build-kyverno-init
build-kyverno-init: $(KYVERNOPRE_BIN) ## Build kyvernopre binary

.PHONY: build-kyverno
build-kyverno: $(KYVERNO_BIN) ## Build kyverno binary

.PHONY: build-cli
build-cli: $(CLI_BIN) ## Build cli binary

.PHONY: build-cleanup-controller
build-cleanup-controller: $(CLEANUP_BIN) ## Build cleanup controller binary

.PHONY: build-reports-controller
build-reports-controller: $(REPORTS_BIN) ## Build reports controller binary

.PHONY: build-background-controller
build-background-controller: $(BACKGROUND_BIN) ## Build background controller binary

build-all: build-kyverno-init build-kyverno build-cli build-cleanup-controller build-reports-controller build-background-controller ## Build all binaries

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
		$(KO) build ./$(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-kyverno
ko-publish-kyverno: ko-login ## Build and publish kyverno image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_KYVERNO) \
		$(KO) build ./$(KYVERNO_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-cli
ko-publish-cli: ko-login ## Build and publish cli image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLI) \
		$(KO) build ./$(CLI_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-cleanup-controller
ko-publish-cleanup-controller: ko-login ## Build and publish cleanup controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_CLEANUP) \
		$(KO) build ./$(CLEANUP_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-reports-controller
ko-publish-reports-controller: ko-login ## Build and publish reports controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_REPORTS) \
		$(KO) build ./$(REPORTS_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

.PHONY: ko-publish-background-controller
ko-publish-background-controller: ko-login ## Build and publish background controller image (with ko)
	@LD_FLAGS=$(LD_FLAGS) KOCACHE=$(KOCACHE) KO_DOCKER_REPO=$(REPO_BACKGROUND) \
		$(KO) build ./$(BACKGROUND_DIR) --bare --tags=$(KO_TAGS) --platform=$(PLATFORMS)

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

GOPATH_SHIM                 := ${PWD}/.gopath
PACKAGE_SHIM                := $(GOPATH_SHIM)/src/$(PACKAGE)
OUT_PACKAGE                 := $(PACKAGE)/pkg/client
INPUT_DIRS                  := $(PACKAGE)/api/kyverno/v1,$(PACKAGE)/api/kyverno/v1beta1,$(PACKAGE)/api/kyverno/v2,$(PACKAGE)/api/kyverno/v2beta1,$(PACKAGE)/api/kyverno/v2alpha1,$(PACKAGE)/api/reports/v1,$(PACKAGE)/api/policyreport/v1alpha2
CLIENT_INPUT_DIRS           := $(PACKAGE)/api/kyverno/v1,$(PACKAGE)/api/kyverno/v2,$(PACKAGE)/api/kyverno/v2alpha1,$(PACKAGE)/api/reports/v1,$(PACKAGE)/api/policyreport/v1alpha2
CLIENTSET_PACKAGE           := $(OUT_PACKAGE)/clientset
LISTERS_PACKAGE             := $(OUT_PACKAGE)/listers
INFORMERS_PACKAGE           := $(OUT_PACKAGE)/informers
APPLYCONFIGURATIONS_PACKAGE := $(OUT_PACKAGE)/applyconfigurations
CRDS_PATH                   := ${PWD}/config/crds
INSTALL_MANIFEST_PATH       := ${PWD}/config/install-latest-testing.yaml
KYVERNO_CHART_VERSION       ?= v0.0.0
POLICIES_CHART_VERSION      ?= v0.0.0
APP_CHART_VERSION           ?= latest
KUBE_CHART_VERSION          ?= ">=1.25.0-0"

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
	@rm -rf $(CLIENTSET_PACKAGE) && mkdir -p $(CLIENTSET_PACKAGE)
	@GOPATH=$(GOPATH_SHIM) $(CLIENT_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--clientset-name versioned \
		--output-package $(CLIENTSET_PACKAGE) \
		--input-base "" \
		--input $(CLIENT_INPUT_DIRS)

.PHONY: codegen-client-listers
codegen-client-listers: $(PACKAGE_SHIM) $(LISTER_GEN) ## Generate listers
	@echo Generate listers... >&2
	@rm -rf $(LISTERS_PACKAGE) && mkdir -p $(LISTERS_PACKAGE)
	@GOPATH=$(GOPATH_SHIM) $(LISTER_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--output-package $(LISTERS_PACKAGE) \
		--input-dirs $(CLIENT_INPUT_DIRS)

.PHONY: codegen-client-informers
codegen-client-informers: $(PACKAGE_SHIM) $(INFORMER_GEN) ## Generate informers
	@echo Generate informers... >&2
	@rm -rf $(INFORMERS_PACKAGE) && mkdir -p $(INFORMERS_PACKAGE)
	@GOPATH=$(GOPATH_SHIM) $(INFORMER_GEN) \
		--go-header-file ./scripts/boilerplate.go.txt \
		--output-package $(INFORMERS_PACKAGE) \
		--input-dirs $(CLIENT_INPUT_DIRS) \
		--versioned-clientset-package $(CLIENTSET_PACKAGE)/versioned \
		--listers-package $(LISTERS_PACKAGE)

.PHONY: codegen-client-wrappers
codegen-client-wrappers: codegen-client-clientset $(GOIMPORTS) ## Generate client wrappers
	@echo Generate client wrappers... >&2
	@go run ./hack/main.go
	@$(GOIMPORTS) -w ./pkg/clients
	@go fmt ./pkg/clients/...

.PHONY: codegen-register
codegen-register: $(PACKAGE_SHIM) $(REGISTER_GEN) ## Generate types registrations
	@echo Generate registration... >&2
	@GOPATH=$(GOPATH_SHIM) $(REGISTER_GEN) \
		--go-header-file=./scripts/boilerplate.go.txt \
		--input-dirs=$(INPUT_DIRS)

.PHONY: codegen-deepcopy
codegen-deepcopy: $(PACKAGE_SHIM) $(DEEPCOPY_GEN) ## Generate deep copy functions
	@echo Generate deep copy functions... >&2
	@GOPATH=$(GOPATH_SHIM) $(DEEPCOPY_GEN) \
		--go-header-file=./scripts/boilerplate.go.txt \
		--input-dirs=$(INPUT_DIRS) \
		--output-file-base=zz_generated.deepcopy

.PHONY: codegen-defaulters
codegen-defaulters: $(PACKAGE_SHIM) $(DEFAULTER_GEN) ## Generate defaulters
	@echo Generate defaulters... >&2
	@GOPATH=$(GOPATH_SHIM) $(DEFAULTER_GEN) --go-header-file=./scripts/boilerplate.go.txt --input-dirs=$(INPUT_DIRS)

.PHONY: codegen-applyconfigurations
codegen-applyconfigurations: $(PACKAGE_SHIM) $(APPLYCONFIGURATION_GEN) ## Generate apply configurations
	@echo Generate applyconfigurations... >&2
	@rm -rf $(APPLYCONFIGURATIONS_PACKAGE) && mkdir -p $(APPLYCONFIGURATIONS_PACKAGE)
	@GOPATH=$(GOPATH_SHIM) $(APPLYCONFIGURATION_GEN) \
		--go-header-file=./scripts/boilerplate.go.txt \
		--input-dirs=$(INPUT_DIRS) \
		--output-package $(APPLYCONFIGURATIONS_PACKAGE)

.PHONY: codegen-client-all
codegen-client-all: ## Generate clientset, listers and informers
codegen-client-all: codegen-register
codegen-client-all: codegen-deepcopy
codegen-client-all: codegen-defaulters
codegen-client-all: codegen-applyconfigurations
codegen-client-all: codegen-client-clientset
codegen-client-all: codegen-client-listers
codegen-client-all: codegen-client-informers
codegen-client-all: codegen-client-wrappers

.PHONY: codegen-crds-kyverno
codegen-crds-kyverno: ## Generate kyverno CRDs
codegen-crds-kyverno: $(PACKAGE_SHIM)
codegen-crds-kyverno: $(CONTROLLER_GEN)
	@echo Generate kyverno crds... >&2
	@rm -rf $(CRDS_PATH)/kyverno && mkdir -p $(CRDS_PATH)/kyverno
	@GOPATH=$(GOPATH_SHIM) $(CONTROLLER_GEN) paths=./api/kyverno/v1/... paths=./api/kyverno/v1beta1/... paths=./api/kyverno/v2/... paths=./api/kyverno/v2alpha1/... paths=./api/kyverno/v2beta1/... crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false output:dir=$(CRDS_PATH)/kyverno

.PHONY: codegen-crds-policyreport
codegen-crds-policyreport: ## Generate policy reports CRDs
codegen-crds-policyreport: $(PACKAGE_SHIM)
codegen-crds-policyreport: $(CONTROLLER_GEN)
	@echo Generate policy reports crds... >&2
	@rm -rf $(CRDS_PATH)/policyreport && mkdir -p $(CRDS_PATH)/policyreport
	@GOPATH=$(GOPATH_SHIM) $(CONTROLLER_GEN) paths=./api/policyreport/... crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false output:dir=$(CRDS_PATH)/policyreport

.PHONY: codegen-crds-reports
codegen-crds-reports: ## Generate reports CRDs
codegen-crds-reports: $(PACKAGE_SHIM)
codegen-crds-reports: $(CONTROLLER_GEN)
	@echo Generate reports crds... >&2
	@rm -rf $(CRDS_PATH)/reports && mkdir -p $(CRDS_PATH)/reports
	@GOPATH=$(GOPATH_SHIM) $(CONTROLLER_GEN) paths=./api/reports/... crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false output:dir=$(CRDS_PATH)/reports

.PHONY: codegen-crds-cli
codegen-crds-cli: ## Generate CLI CRDs
codegen-crds-cli: $(PACKAGE_SHIM)
codegen-crds-cli: $(CONTROLLER_GEN)
	@echo Generate cli crds... >&2
	@rm -rf ${PWD}/cmd/cli/kubectl-kyverno/config/crds && mkdir -p ${PWD}/cmd/cli/kubectl-kyverno/config/crds
	@GOPATH=$(GOPATH_SHIM) $(CONTROLLER_GEN) paths=./cmd/cli/kubectl-kyverno/apis/... crd:crdVersions=v1,ignoreUnexportedFields=true,generateEmbeddedObjectMeta=false output:dir=${PWD}/cmd/cli/kubectl-kyverno/config/crds

.PHONY: codegen-crds-all
codegen-crds-all: codegen-crds-kyverno codegen-crds-policyreport codegen-crds-reports codegen-cli-crds ## Generate all CRDs

.PHONY: codegen-helm-docs
codegen-helm-docs: ## Generate helm docs
	@echo Generate helm docs... >&2
	@docker run -v ${PWD}/charts:/work -w /work jnorwood/helm-docs:v1.11.0 -s file

.PHONY: codegen-api-docs
codegen-api-docs: $(PACKAGE_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) $(GENREF) ## Generate API docs
	@echo Generate api docs... >&2
	@rm -rf docs/user/crd && mkdir -p docs/user/crd
	@GOPATH=$(GOPATH_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) -v 4 \
		-api-dir $(PACKAGE)/api \
		-config docs/user/config.json \
		-template-dir docs/user/template \
		-out-file docs/user/crd/index.html
	@cd ./docs/user && GOPATH=$(GOPATH_SHIM) $(GENREF) \
		-c config-api.yaml \
		-o crd \
		-f html

.PHONY: codegen-cli-api-docs
codegen-cli-api-docs: $(PACKAGE_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) $(GENREF) ## Generate CLI API docs
	@echo Generate CLI api docs... >&2
	@rm -rf docs/user/cli/crd && mkdir -p docs/user/cli/crd
	@GOPATH=$(GOPATH_SHIM) $(GEN_CRD_API_REFERENCE_DOCS) -v 4 \
		-api-dir $(PACKAGE)/cmd/cli/kubectl-kyverno/apis \
		-config docs/user/config.json \
		-template-dir docs/user/template \
		-out-file docs/user/cli/crd/index.html
	@cd ./docs/user && GOPATH=$(GOPATH_SHIM) $(GENREF) \
		-c config-cli-api.yaml \
		-o cli/crd \
		-f html

.PHONY: codegen-cli-docs
codegen-cli-docs: $(CLI_BIN) ## Generate CLI docs
	@echo Generate cli docs... >&2
	@rm -rf docs/user/cli/commands && mkdir -p docs/user/cli/commands
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) docs -o docs/user/cli/commands --autogenTag=false

.PHONY: codegen-cli-crds
codegen-cli-crds: codegen-crds-kyverno ## Copy generated CRDs to embed in the CLI
	@echo Copy generated CRDs to embed in the CLI... >&2
	@rm -rf cmd/cli/kubectl-kyverno/data/crds && mkdir -p cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_clusterpolicies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_policies.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp config/crds/kyverno/kyverno.io_policyexceptions.yaml cmd/cli/kubectl-kyverno/data/crds
	@cp cmd/cli/kubectl-kyverno/config/crds/* cmd/cli/kubectl-kyverno/data/crds

.PHONY: codegen-docs-all
codegen-docs-all: codegen-helm-docs codegen-cli-docs codegen-api-docs codegen-cli-api-docs ## Generate all docs

.PHONY: codegen-fix-tests
codegen-fix-tests: $(CLI_BIN) ## Fix CLI test files
	@echo Fix CLI test files... >&2
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) fix test . --save --compress --force

.PHONY: codegen-fix-policies
codegen-fix-policies: $(CLI_BIN) ## Fix CLI policy files
	@echo Fix CLI policy files... >&2
	@KYVERNO_EXPERIMENTAL=true $(CLI_BIN) fix policy . --save

.PHONY: codegen-cli-all
codegen-cli-all: codegen-cli-crds codegen-cli-docs codegen-cli-api-docs codegen-fix-tests ## Generate all CLI related code and docs

define generate_crd
	@echo "{{- if .Values.groups.$(4).$(5) }}" > ./charts/kyverno/charts/crds/templates/$(3)/$(1)
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

.PHONY: codegen-helm-crds
codegen-helm-crds: codegen-crds-all ## Generate helm CRDs
	@echo Generate helm crds... >&2
	@rm -rf ./charts/kyverno/charts/crds/templates/kyverno.io && mkdir -p ./charts/kyverno/charts/crds/templates/kyverno.io
	@rm -rf ./charts/kyverno/charts/crds/templates/reports.kyverno.io && mkdir -p ./charts/kyverno/charts/crds/templates/reports.kyverno.io
	@rm -rf ./charts/kyverno/charts/crds/templates/wgpolicyk8s.io && mkdir -p ./charts/kyverno/charts/crds/templates/wgpolicyk8s.io
	$(call generate_crd,kyverno.io_cleanuppolicies.yaml,kyverno,kyverno.io,kyverno,cleanuppolicies)
	$(call generate_crd,kyverno.io_clustercleanuppolicies.yaml,kyverno,kyverno.io,kyverno,clustercleanuppolicies)
	$(call generate_crd,kyverno.io_clusterpolicies.yaml,kyverno,kyverno.io,kyverno,clusterpolicies)
	$(call generate_crd,kyverno.io_globalcontextentries.yaml,kyverno,kyverno.io,kyverno,globalcontextentries)
	$(call generate_crd,kyverno.io_policies.yaml,kyverno,kyverno.io,kyverno,policies)
	$(call generate_crd,kyverno.io_policyexceptions.yaml,kyverno,kyverno.io,kyverno,policyexceptions)
	$(call generate_crd,kyverno.io_updaterequests.yaml,kyverno,kyverno.io,kyverno,updaterequests)
	$(call generate_crd,reports.kyverno.io_clusterephemeralreports.yaml,reports,reports.kyverno.io,reports,clusterephemeralreports)
	$(call generate_crd,reports.kyverno.io_ephemeralreports.yaml,reports,reports.kyverno.io,reports,ephemeralreports)
	$(call generate_crd,wgpolicyk8s.io_clusterpolicyreports.yaml,policyreport,wgpolicyk8s.io,wgpolicyk8s,clusterpolicyreports)
	$(call generate_crd,wgpolicyk8s.io_policyreports.yaml,policyreport,wgpolicyk8s.io,wgpolicyk8s,policyreports)

.PHONY: codegen-helm-all
codegen-helm-all: codegen-helm-crds codegen-helm-docs ## Generate helm docs and CRDs

.PHONY: codegen-manifest-install-latest
codegen-manifest-install-latest: $(HELM) ## Create install_latest manifest
	@echo Generate latest install manifest... >&2
	@$(HELM) template kyverno --kube-version $(KUBE_VERSION) --namespace kyverno --skip-tests ./charts/kyverno \
		--set templating.enabled=true \
		--set templating.version=latest \
		--set admissionController.container.image.tag=latest \
		--set admissionController.initContainer.image.tag=latest \
		--set cleanupController.image.tag=latest \
		--set reportsController.image.tag=latest \
		--set backgroundController.image.tag=latest \
 		| $(SED) -e '/^#.*/d' \
		> ./config/install-latest-testing.yaml

.PHONY: codegen-manifest-debug
codegen-manifest-debug: $(HELM) ## Create debug manifest
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
codegen-manifest-release: $(HELM) ## Create release manifest
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
codegen-manifest-all: codegen-manifest-install-latest codegen-manifest-debug ## Create all manifests

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

.PHONY: codegen-quick
codegen-quick: ## Generate all generated code except client
codegen-quick: codegen-deepcopy
codegen-quick: codegen-crds-all
codegen-quick: codegen-docs-all
codegen-quick: codegen-helm-all
codegen-quick: codegen-manifest-all

.PHONY: codegen-slow
codegen-slow: ## Generate client code
codegen-slow: codegen-client-all

.PHONY: codegen-all
codegen-all: ## Generate all generated code
codegen-all: codegen-quick
codegen-all: codegen-slow

##################
# VERIFY CODEGEN #
##################

.PHONY: verify-crds
verify-crds: ## Check CRDs are up to date
verify-crds: codegen-crds-all
	@echo Checking crds are up to date... >&2
	@git --no-pager diff $(CRDS_PATH)
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-crds-all".' >&2
	@echo 'To correct this, locally run "make codegen-crds-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code $(CRDS_PATH)

.PHONY: verify-client
verify-client: ## Check client is up to date
verify-client: codegen-client-all
	@echo Checking client is up to date... >&2
	@git --no-pager diff --ignore-space-change pkg/client
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-client-all".' >&2
	@echo 'To correct this, locally run "make codegen-client-all", commit the changes, and re-run tests.' >&2
	@git diff --ignore-space-change --quiet --exit-code pkg/client
	@git --no-pager diff --ignore-space-change pkg/clients
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-client-all".' >&2
	@echo 'To correct this, locally run "make codegen-client-all", commit the changes, and re-run tests.' >&2
	@git diff --ignore-space-change --quiet --exit-code pkg/clients

.PHONY: verify-deepcopy
verify-deepcopy: ## Check deepcopy functions are up to date
verify-deepcopy: codegen-deepcopy
	@echo Checking deepcopy functions are up to date... >&2
	@git --no-pager diff api
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-deepcopy".' >&2
	@echo 'To correct this, locally run "make codegen-deepcopy", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code api

.PHONY: verify-docs
verify-docs: ## Check docs are up to date
verify-docs: codegen-docs-all
	@echo Checking docs are up to date... >&2
	@git --no-pager diff docs/user
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-docs-all".' >&2
	@echo 'To correct this, locally run "make codegen-docs-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code docs/user

.PHONY: verify-helm
verify-helm: ## Check Helm charts are up to date
verify-helm: codegen-helm-all
	@echo Checking helm charts are up to date... >&2
	@git --no-pager diff charts
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-helm-all".' >&2
	@echo 'To correct this, locally run "make codegen-helm-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code charts

.PHONY: verify-manifests
verify-manifests: ## Check manifests are up to date
verify-manifests: codegen-manifest-all
	@echo Checking manifests are up to date... >&2
	@git --no-pager diff ${INSTALL_MANIFEST_PATH}
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-manifest-all".' >&2
	@echo 'To correct this, locally run "make codegen-manifest-all", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code ${INSTALL_MANIFEST_PATH}

.PHONY: verify-cli-crds
verify-cli-crds: ## Check generated CRDs to be embedded in the CLI are up to date
verify-cli-crds: codegen-cli-crds
	@echo Checking generated CRDs to be embedded in the CLI are up to date... >&2
	@git --no-pager diff cmd/cli/kubectl-kyverno/data/crds
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-cli-crds".' >&2
	@echo 'To correct this, locally run "make codegen-cli-crds", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code cmd/cli/kubectl-kyverno/data/crds

.PHONY: verify-cli-tests
verify-cli-tests: ## Check CLI test files are up to date
	@echo Checking CLI test files are up to date... >&2
	@git --no-pager diff test/cli
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen-fix-tests".' >&2
	@echo 'To correct this, locally run "make codegen-fix-tests", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code test/cli

.PHONY: verify-codegen
verify-codegen: ## Verify all generated code and docs are up to date
verify-codegen: verify-crds
verify-codegen: verify-client
verify-codegen: verify-deepcopy
verify-codegen: verify-docs
verify-codegen: verify-helm
verify-codegen: verify-manifests
verify-codegen: verify-cli-crds

##############
# UNIT TESTS #
##############

CODE_COVERAGE_FILE      := coverage
CODE_COVERAGE_FILE_TXT  := $(CODE_COVERAGE_FILE).txt
CODE_COVERAGE_FILE_HTML := $(CODE_COVERAGE_FILE).html

.PHONY: test
test: test-clean test-unit ## Clean tests cache then run unit tests

.PHONY: test-clean
test-clean: ## Clean tests cache
	@echo Clean test cache... >&2
	@go clean -testcache

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
test-cli-local: test-cli-local-validate test-cli-local-mutate test-cli-local-generate test-cli-local-registry test-cli-local-scenarios test-cli-local-selector ## Run local CLI tests

.PHONY: test-cli-local-validate
test-cli-local-validate: $(CLI_BIN) ## Run local CLI validation tests
	@echo Running local cli validation tests... >&2
	@$(CLI_BIN) test ./test/cli/test

.PHONY: test-cli-local-mutate
test-cli-local-mutate: $(CLI_BIN) ## Run local CLI mutation tests
	@echo Running local cli mutation tests... >&2
	@$(CLI_BIN) test ./test/cli/test-mutate

.PHONY: test-cli-local-generate
test-cli-local-generate: $(CLI_BIN) ## Run local CLI generation tests
	@echo Running local cli generation tests... >&2
	@$(CLI_BIN) test ./test/cli/test-generate

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

#############
# PERF TEST #
#############

PERF_TEST_NODE_COUNT		?= 3
PERF_TEST_MEMORY_REQUEST	?= "1Gi"

.PHONY: test-perf
test-perf: $(PACKAGE_SHIM) ## Run perf tests
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
kind-install-kyverno: $(HELM) ## Install kyverno helm chart
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
