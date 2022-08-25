.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################

GIT_VERSION := $(shell git describe --match "v[0-9]*" --tags $(git rev-list --tags --max-count=1))
GIT_VERSION_DEV := $(shell git describe --match "[0-9].[0-9]-dev*")
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')
VERSION ?= $(shell git describe --match "v[0-9]*")

REGISTRY?=ghcr.io
REPO=$(REGISTRY)/kyverno
IMAGE_TAG_LATEST_DEV=$(shell git describe --match "[0-9].[0-9]-dev*" | cut -d '-' -f-2)
IMAGE_TAG_DEV=$(GIT_VERSION_DEV)
IMAGE_TAG?=$(GIT_VERSION)
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
ifeq ($(GOOS), darwin)
SED=gsed
else
SED=sed
endif
PACKAGE ?=github.com/kyverno/kyverno
export LD_FLAGS = -s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)
export LD_FLAGS_DEV = -s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION_DEV) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)
K8S_VERSION ?= $(shell kubectl version --short | grep -i server | cut -d" " -f3 | cut -c2-)
export K8S_VERSION
TEST_GIT_BRANCH ?= main

KIND_IMAGE?=kindest/node:v1.24.0

#########
# TOOLS #
#########

TOOLS_DIR                          := $(PWD)/.tools
KIND                               := $(TOOLS_DIR)/kind
KIND_VERSION                       := v0.14.0
CONTROLLER_GEN                     := $(TOOLS_DIR)/controller-gen
CONTROLLER_GEN_VERSION             := v0.9.1-0.20220629131006-1878064c4cdf
GEN_CRD_API_REFERENCE_DOCS         := $(TOOLS_DIR)/gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS_VERSION := latest
GO_ACC                             := $(TOOLS_DIR)/go-acc
GO_ACC_VERSION                     := latest
KUSTOMIZE                          := $(TOOLS_DIR)/kustomize
KUSTOMIZE_VERSION                  := latest
GOIMPORTS                          := $(TOOLS_DIR)/goimports
GOIMPORTS_VERSION                  := latest
HELM_DOCS                          := $(TOOLS_DIR)/helm-docs
HELM_DOCS_VERSION                  := v1.6.0
KO                                 := $(TOOLS_DIR)/ko
KO_VERSION                         := v0.12.0
TOOLS                              := $(KIND) $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GO_ACC) $(KUSTOMIZE) $(GOIMPORTS) $(HELM_DOCS) $(KO)

$(KIND):
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kind@$(KIND_VERSION)

$(CONTROLLER_GEN):
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(GEN_CRD_API_REFERENCE_DOCS):
	@GOBIN=$(TOOLS_DIR) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

$(GO_ACC):
	@GOBIN=$(TOOLS_DIR) go install github.com/ory/go-acc@$(GO_ACC_VERSION)

$(KUSTOMIZE):
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION)

$(GOIMPORTS):
	@GOBIN=$(TOOLS_DIR) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

$(HELM_DOCS):
	@GOBIN=$(TOOLS_DIR) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

$(KO):
	@GOBIN=$(TOOLS_DIR) go install github.com/google/ko@$(KO_VERSION)

.PHONY: install-tools
install-tools: $(TOOLS) ## Install tools

.PHONY: clean-tools
clean-tools: ## Remove tools
	@rm -rf $(TOOLS_DIR)

##################################
# KYVERNO
##################################

.PHONY: unused-package-check
unused-package-check:
	@echo "------------------"
	@echo "--> Check unused packages for the all kyverno components"
	@echo "------------------"
	@tidy=$$(go mod tidy); \
	if [ -n "$${tidy}" ]; then \
		echo "go mod tidy checking failed!"; echo "$${tidy}"; echo; \
	fi

KYVERNO_PATH:= cmd/kyverno
build: kyverno
PWD := $(CURDIR)

##################################
# INIT CONTAINER
##################################

INITC_PATH := cmd/initContainer
INITC_IMAGE := kyvernopre
initContainer: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags="$(LD_FLAGS)" $(PWD)/$(INITC_PATH)

.PHONY: ko-build-initContainer

ko-build-initContainer: KO_DOCKER_REPO=$(REPO)/$(INITC_IMAGE)
ko-build-initContainer:
	@ko build ./$(INITC_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64,linux/arm64,linux/s390x

ko-build-initContainer-amd64: KO_DOCKER_REPO=$(REPO)/$(INITC_IMAGE)
ko-build-initContainer-amd64:
	@ko build ./$(INITC_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64

ko-build-initContainer-local: KO_DOCKER_REPO=kind.local
ko-build-initContainer-local: kind-e2e-cluster
	@ko build ./$(INITC_PATH) --platform=linux/$(GOARCH) --tags=latest,$(IMAGE_TAG_DEV) --preserve-import-paths
INITC_KIND_IMAGE = kind.local/github.com/kyverno/kyverno/cmd/initcontainer

# TODO(jason): LD_FLAGS_DEV
ko-build-initContainer-dev: KO_DOCKER_REPO=$(REPO)/$(INITC_IMAGE)
ko-build-initContainer-dev:
	@ko build ./$(INITC_PATH) --bare --platform=linux/amd64,linux/arm64,linux/s390x --tags=latest,$(IMAGE_TAG_DEV),$(IMAGE_TAG_LATEST_DEV)

##################################
# KYVERNO CONTAINER
##################################

.PHONY: ko-build-kyverno
KYVERNO_PATH := cmd/kyverno
KYVERNO_IMAGE := kyverno

kyverno: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags"$(LD_FLAGS)" $(PWD)/$(KYVERNO_PATH)

ko-build-kyverno: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_IMAGE)
ko-build-kyverno:
	@ko build ./$(KYVERNO_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64,linux/arm64,linux/s390x

ko-build-kyverno-amd64: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_IMAGE)
ko-build-kyverno-amd64:
	@ko build ./$(KYVERNO_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64

ko-build-kyverno-local: KO_DOCKER_REPO=kind.local
ko-build-kyverno-local: kind-e2e-cluster
	@ko build ./$(KYVERNO_PATH) --platform=linux/$(GOARCH) --tags=latest,$(IMAGE_TAG_DEV) --preserve-import-paths

KYVERNO_KIND_IMAGE = kind.local/github.com/kyverno/kyverno/cmd/kyverno

# TODO(jason): LD_FLAGS_DEV
ko-build-kyverno-dev: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_IMAGE)
ko-build-kyverno-dev:
	@ko build ./$(KYVERNO_PATH) --bare --platform=linux/amd64,linux/arm64,linux/s390x --tags=latest,$(IMAGE_TAG_DEV),$(IMAGE_TAG_LATEST_DEV)

##################################
# Generate Docs for types.go
##################################

.PHONY: generate-api-docs
generate-api-docs: $(GEN_CRD_API_REFERENCE_DOCS) ## Generate api reference docs
	rm -rf docs/crd
	mkdir docs/crd
	$(GEN_CRD_API_REFERENCE_DOCS) -v 6 -api-dir ./api/kyverno/v1alpha2 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1alpha2/index.html
	$(GEN_CRD_API_REFERENCE_DOCS) -v 6 -api-dir ./api/kyverno/v1beta1 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1beta1/index.html
	$(GEN_CRD_API_REFERENCE_DOCS) -v 6 -api-dir ./api/kyverno/v1 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1/index.html

.PHONY: verify-api-docs
verify-api-docs: generate-api-docs ## Check api reference docs are up to date
	git --no-pager diff docs
	@echo 'If this test fails, it is because the git diff is non-empty after running "make generate-api-docs".'
	@echo 'To correct this, locally run "make generate-api-docs", commit the changes, and re-run tests.'
	git diff --quiet --exit-code docs

##################################
# CLI
##################################
.PHONY: ko-build-cli
CLI_PATH := cmd/cli/kubectl-kyverno
KYVERNO_CLI_IMAGE := kyverno-cli

cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyverno -ldflags="$(LD_FLAGS)" $(PWD)/$(CLI_PATH)

ko-build-cli: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_CLI_IMAGE)
ko-build-cli:
	@ko build ./$(CLI_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64,linux/arm64,linux/s390x

ko-build-cli-amd64: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_CLI_IMAGE)
ko-build-cli-amd64:
	@ko build ./$(CLI_PATH) --bare --tags=latest,$(IMAGE_TAG) --platform=linux/amd64

ko-build-cli-local: KO_DOCKER_REPO=ko.local
ko-build-cli-local:
	@ko build ./$(CLI_PATH) --platform=linux/$(GOARCH) --tags=latest,$(IMAGE_TAG_DEV)

# TODO(jason): LD_FLAGS_DEV
ko-build-cli-dev: KO_DOCKER_REPO=$(REPO)/$(KYVERNO_CLI_IMAGE)
ko-build-cli-dev:
	@ko build ./$(CLI_PATH) --bare --platform=linux/amd64,linux/arm64,linux/s390x --tags=latest,$(IMAGE_TAG_DEV),$(IMAGE_TAG_LATEST_DEV)

##################################
ko-build-all: ko-build-initContainer ko-build-kyverno ko-build-cli

ko-build-all-amd64: ko-build-initContainer-amd64 ko-build-kyverno-amd64 ko-build-cli-amd64

##################################
# Create e2e Infrastructure
##################################

.PHONY: kind-e2e-cluster
kind-e2e-cluster: $(KIND) ## Create kind cluster for e2e tests
	$(KIND) create cluster --image=$(KIND_IMAGE)

.PHONY: e2e-kustomize
e2e-kustomize: $(KUSTOMIZE) ## Build kustomize manifests for e2e tests
	cd config && \
	kustomize edit set image $(INITC_KIND_IMAGE):$(IMAGE_TAG_DEV) && \
	kustomize edit set image $(KYVERNO_KIND_IMAGE):$(IMAGE_TAG_DEV)
	kustomize build config/ -o config/install.yaml

.PHONY: create-e2e-infrastructure
create-e2e-infrastructure: ko-build-initContainer-local ko-build-kyverno-local e2e-kustomize ## Setup infrastructure for e2e tests

##################################
# Testing & Code-Coverage
##################################

CODE_COVERAGE_FILE:= coverage
CODE_COVERAGE_FILE_TXT := $(CODE_COVERAGE_FILE).txt
CODE_COVERAGE_FILE_HTML := $(CODE_COVERAGE_FILE).html

test: test-clean test-unit test-e2e ## Clean tests cache then run unit and e2e tests

test-clean: ## Clean tests cache
	@echo "	cleaning test cache"
	go clean -testcache ./...

.PHONY: test-cli
test-cli: test-cli-policies test-cli-local test-cli-local-mutate test-cli-local-generate test-cli-test-case-selector-flag test-cli-registry

.PHONY: test-cli-policies
test-cli-policies: cli
	cmd/cli/kubectl-kyverno/kyverno test https://github.com/kyverno/policies/$(TEST_GIT_BRANCH)

.PHONY: test-cli-local
test-cli-local: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test

.PHONY: test-cli-local-mutate
test-cli-local-mutate: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test-mutate

.PHONY: test-cli-local-generate
test-cli-local-generate: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test-generate

.PHONY: test-cli-test-case-selector-flag
test-cli-test-case-selector-flag: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

.PHONY: test-cli-registry
test-cli-registry: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/registry --registry

test-unit: $(GO_ACC) ## Run unit tests
	@echo "	running unit tests"
	$(GO_ACC) ./... -o $(CODE_COVERAGE_FILE_TXT)

code-cov-report: ## Generate code coverage report
	@echo "	generating code coverage report"
	GO111MODULE=on go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out -o $(CODE_COVERAGE_FILE_TXT)
	go tool cover -html=coverage.out -o $(CODE_COVERAGE_FILE_HTML)

# Test E2E
test-e2e:
	$(eval export E2E="ok")
	go test ./test/e2e/verifyimages -v
	go test ./test/e2e/metrics -v
	go test ./test/e2e/mutate -v
	go test ./test/e2e/generate -v
	$(eval export E2E="")

test-e2e-local:
	$(eval export E2E="ok")
	kubectl apply -f https://raw.githubusercontent.com/kyverno/kyverno/main/config/github/rbac.yaml
	kubectl port-forward -n kyverno service/kyverno-svc-metrics  8000:8000 &
	go test ./test/e2e/verifyimages -v
	go test ./test/e2e/metrics -v
	go test ./test/e2e/mutate -v
	go test ./test/e2e/generate -v
	kill  $!
	$(eval export E2E="")

helm-test-values:
	sed -i -e "s|nameOverride:.*|nameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|fullnameOverride:.*|fullnameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|namespace:.*|namespace: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|tag:  # replaced in e2e tests.*|tag: $(IMAGE_TAG_DEV)|" charts/kyverno/values.yaml
	sed -i -e "s|repository: ghcr.io/kyverno/kyvernopre  # init: replaced in e2e tests|repository: $(INITC_KIND_IMAGE)|" charts/kyverno/values.yaml
	sed -i -e "s|repository: ghcr.io/kyverno/kyverno  # kyverno: replaced in e2e tests|repository: $(KYVERNO_KIND_IMAGE)|" charts/kyverno/values.yaml

# godownloader create downloading script for kyverno-cli
godownloader:
	godownloader .goreleaser.yml --repo kyverno/kyverno -o ./scripts/install-cli.sh  --source="raw"

.PHONY: kustomize-crd
kustomize-crd: $(KUSTOMIZE) ## Create install.yaml
	# Create CRD for helm deployment Helm
	$(KUSTOMIZE) build ./config/release | kustomize cfg grep kind=CustomResourceDefinition | $(SED) -e "1i{{- if .Values.installCRDs }}" -e '$$a{{- end }}' > ./charts/kyverno/templates/crds.yaml
	# Generate install.yaml that have all resources for kyverno
	$(KUSTOMIZE) build ./config > ./config/install.yaml
	# Generate install_debug.yaml that for developer testing
	$(KUSTOMIZE) build ./config/debug > ./config/install_debug.yaml

# guidance https://github.com/kyverno/kyverno/wiki/Generate-a-Release
release:
	$(KUSTOMIZE) build ./config > ./config/install.yaml
	$(KUSTOMIZE) build ./config/release > ./config/release/install.yaml

release-notes:
	@bash -c 'while IFS= read -r line ; do if [[ "$$line" == "## "* && "$$line" != "## $(VERSION)" ]]; then break ; fi; echo "$$line"; done < "CHANGELOG.md"' \
	true

##################################
# CODEGEN
##################################

.PHONY: kyverno-crd
kyverno-crd: $(CONTROLLER_GEN) ## Generate Kyverno CRDs
	$(CONTROLLER_GEN) crd paths=./api/kyverno/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: report-crd
report-crd: $(CONTROLLER_GEN) ## Generate policy reports CRDs
	$(CONTROLLER_GEN) crd paths=./api/policyreport/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: deepcopy-autogen
deepcopy-autogen: $(CONTROLLER_GEN) $(GOIMPORTS) ## Generate deep copy code
	$(CONTROLLER_GEN) object:headerFile="scripts/boilerplate.go.txt" paths="./..." && $(GOIMPORTS) -w ./api/

.PHONY: codegen
codegen: kyverno-crd report-crd deepcopy-autogen generate-api-docs gen-helm ## Update all generated code and docs

.PHONY: verify-api
verify-api: kyverno-crd report-crd deepcopy-autogen ## Check api is up to date
	git --no-pager diff api
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen".'
	@echo 'To correct this, locally run "make codegen", commit the changes, and re-run tests.'
	git diff --quiet --exit-code api

.PHONY: verify-config
verify-config: kyverno-crd report-crd ## Check config is up to date
	git --no-pager diff config
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen".'
	@echo 'To correct this, locally run "make codegen", commit the changes, and re-run tests.'
	git diff --quiet --exit-code config

.PHONY: verify-codegen
verify-codegen: verify-api verify-config verify-api-docs verify-helm ## Verify all generated code and docs are up to date

.PHONY: fmt
fmt: $(GOIMPORTS) ## Run go fmt
	go fmt ./... && $(GOIMPORTS) -w ./

.PHONY: vet
vet: ## Run go vet
	go vet ./...

##################################
# HELM
##################################

.PHONY: gen-helm-docs
gen-helm-docs: $(HELM_DOCS) ## Generate Helm docs
	# @$(HELM_DOCS) -s file
	@docker run -v ${PWD}:/work -w /work jnorwood/helm-docs:v1.6.0 -s file

.PHONY: gen-helm
gen-helm: gen-helm-docs kustomize-crd ## Generate Helm charts stuff

.PHONY: verify-helm
verify-helm: gen-helm ## Check Helm charts are up to date
	git --no-pager diff charts
	@echo 'If this test fails, it is because the git diff is non-empty after running "make gen-helm".'
	@echo 'To correct this, locally run "make gen-helm", commit the changes, and re-run tests.'
	git diff --quiet --exit-code charts

##################################
# HELP
##################################

.PHONY: help
help: ## Shows the available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: kind-deploy
kind-deploy: ko-build-initContainer-local ko-build-kyverno-local
	helm upgrade --install kyverno --namespace kyverno --wait --create-namespace ./charts/kyverno \
		--set image.repository=$(KYVERNO_KIND_IMAGE) \
		--set image.tag=$(IMAGE_TAG_DEV) \
		--set initImage.repository=$(INITC_KIND_IMAGE) \
		--set initImage.tag=$(IMAGE_TAG_DEV) \
		--set extraArgs={--autogenInternals=true}
	helm upgrade --install kyverno-policies --namespace kyverno --create-namespace ./charts/kyverno-policies

