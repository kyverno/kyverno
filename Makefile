.DEFAULT_GOAL: build

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
REPO                  = $(REGISTRY)/kyverno
IMAGE_TAG_LATEST_DEV  = $(shell git describe --match "[0-9].[0-9]-dev*" | cut -d '-' -f-2)
IMAGE_TAG_DEV         = $(GIT_VERSION_DEV)
IMAGE_TAG            ?= $(GIT_VERSION)
K8S_VERSION          ?= $(shell kubectl version --short | grep -i server | cut -d" " -f3 | cut -c2-)
TEST_GIT_BRANCH      ?= main
KIND_IMAGE           ?= kindest/node:v1.24.0

export K8S_VERSION

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
ifeq ($(GOOS), darwin)
SED                                := gsed
else
SED                                := sed
endif

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
clean-tools: ## Remove installed tools
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
GOOS           ?= $(shell go env GOOS)
GOARCH         ?= $(shell go env GOARCH)
CGO_ENABLED    ?= 0 
LD_FLAGS        = "-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"
LD_FLAGS_DEV    = "-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION_DEV) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"

.PHONY: fmt
fmt: ## Run go fmt
	@go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@go vet ./...

$(KYVERNO_BIN): fmt vet
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(KYVERNO_BIN) -ldflags=$(LD_FLAGS) $(KYVERNO_DIR)

$(KYVERNOPRE_BIN): fmt vet
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(KYVERNOPRE_BIN) -ldflags=$(LD_FLAGS) $(KYVERNOPRE_DIR)

$(CLI_BIN): fmt vet
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build -o $(CLI_BIN) -ldflags=$(LD_FLAGS) $(CLI_DIR)

.PHONY: build-kyverno
build-kyverno: $(KYVERNO_BIN) ## Build kyverno binary

.PHONY: build-kyvernopre
build-kyvernopre: $(KYVERNOPRE_BIN) ## Build kyvernopre binary

.PHONY: build-cli
build-cli: $(CLI_BIN) ## Build CLI binary

build-all: build-kyverno build-kyvernopre build-cli ## Build all binaries

##############
# BUILD (KO) #
##############

KO_PLATFORM         := linux/amd64,linux/arm64,linux/s390x
KO_TAGS             := latest,$(IMAGE_TAG)
KO_TAGS_DEV         := latest,$(IMAGE_TAG_DEV)
KYVERNOPRE_IMAGE    := kyvernopre
KYVERNO_IMAGE       := kyverno
CLI_IMAGE           := kyverno-cli

.PHONY: ko-build-kyvernopre
ko-build-kyvernopre: $(KO) ## Build kyvernopre local image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=ko.local $(KO) build $(KYVERNOPRE_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-build-kyverno
ko-build-kyverno: $(KO) ## Build kyverno local image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=ko.local $(KO) build $(KYVERNO_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-build-cli
ko-build-cli: $(KO) ## Build CLI local image (with ko)
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=ko.local $(KO) build $(CLI_DIR) --preserve-import-paths --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-build-all
ko-build-all: ko-build-kyvernopre ko-build-kyverno ko-build-cli ## Build all local images (with ko)

################
# PUBLISH (KO) #
################

REPO_KYVERNOPRE     := $(REPO)/$(KYVERNOPRE_IMAGE)
REPO_KYVERNO        := $(REPO)/$(KYVERNO_IMAGE)
REPO_CLI            := $(REPO)/$(CLI_IMAGE)
REGISTRY_USERNAME	?= dummy
INITC_KIND_IMAGE    := ko.local/github.com/kyverno/kyverno/cmd/initcontainer
KYVERNO_KIND_IMAGE  := ko.local/github.com/kyverno/kyverno/cmd/kyverno

.PHONY: ko-login
ko-login: $(KO)
	@$(KO) login $(REGISTRY) --username $(REGISTRY_USERNAME) --password $(REGISTRY_PASSWORD)

.PHONY: ko-publish-kyvernopre
ko-publish-kyvernopre: ko-login
	@LD_FLAGS=$(LD_FLAGS) KO_DOCKER_REPO=$(REPO_KYVERNOPRE) $(KO) build $(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-kyverno
ko-publish-kyverno: ko-login
	@LD_FLAGS=$(LD_FLAGS) KO_DOCKER_REPO=$(REPO_KYVERNO) $(KO) build $(KYVERNO_DIR) --bare --tags=$(KO_TAGS) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-cli
ko-publish-cli: ko-login
	@LD_FLAGS=$(LD_FLAGS) KO_DOCKER_REPO=$(REPO_CLI) $(KO) build $(CLI_DIR) --bare --tags=$(KO_TAGS) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-kyvernopre-dev
ko-publish-kyvernopre-dev: ko-login
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=$(REPO_KYVERNOPRE) $(KO) build $(KYVERNOPRE_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-kyverno-dev
ko-publish-kyverno-dev: ko-login
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=$(REPO_KYVERNO) $(KO) build $(KYVERNO_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-cli-dev
ko-publish-cli-dev: ko-login
	@LD_FLAGS=$(LD_FLAGS_DEV) KO_DOCKER_REPO=$(REPO_CLI) $(KO) build $(CLI_DIR) --bare --tags=$(KO_TAGS_DEV) --platform=$(KO_PLATFORM)

.PHONY: ko-publish-all
ko-publish-all: ko-publish-kyvernopre ko-publish-kyverno ko-publish-cli

.PHONY: ko-publish-all-dev
ko-publish-all-dev: ko-publish-kyvernopre-dev ko-publish-kyverno-dev ko-publish-cli-dev

##################
# BUILD (DOCKER) #
##################

.PHONY: docker-buildx-builder
docker-buildx-builder:
	if ! docker buildx ls | grep -q kyverno; then\
		docker buildx create --name kyverno --use;\
	fi

.PHONY: docker-build-initContainer
docker-build-initContainer: docker-buildx-builder
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plane --platform $(KO_PLATFORM) --tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-push-initContainer
docker-push-initContainer: docker-buildx-builder
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plane --push --platform $(KO_PLATFORM) --tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-push-initContainer-dev
docker-build-initContainer-dev: docker-buildx-builder
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plane --platform $(KO_PLATFORM) \
		--tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV) --tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO)/$(KYVERNOPRE_IMAGE):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-push-initContainer-dev
docker-push-initContainer-dev: docker-buildx-builder
	@docker buildx build --file $(KYVERNOPRE_DIR)/Dockerfile --progress plane --push --platform $(KO_PLATFORM) \
		--tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV) --tag $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO)/$(KYVERNOPRE_IMAGE):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-build-kyverno
docker-build-kyverno: docker-buildx-builder
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plane --platform $(KO_PLATFORM) --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-push-kyverno
docker-push-kyverno: docker-buildx-builder
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plane --push --platform $(KO_PLATFORM) --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-push-kyverno-dev
docker-push-kyverno-dev: docker-buildx-builder
	@docker buildx build --file $(KYVERNO_DIR)/Dockerfile --progress plane --push --platform $(KO_PLATFORM) \
		--tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest --tag $(REPO)/$(KYVERNO_IMAGE):latest \
		. --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

.PHONY: docker-build-cli
docker-build-cli: docker-buildx-builder
	@docker buildx build --file $(CLI_DIR)/Dockerfile --progress plane --platform $(KO_PLATFORM) --tag $(REPO)/$(CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-push-cli
docker-push-cli: docker-buildx-builder
	@docker buildx build --file $(CLI_DIR)/Dockerfile --progress plane --push --platform $(KO_PLATFORM) --tag $(REPO)/$(CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

.PHONY: docker-get-initContainer-digest
docker-get-initContainer-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-initContainer-digest-dev
docker-get-initContainer-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNOPRE_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-kyverno-digest
docker-get-kyverno-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

.PHONY: docker-get-kyverno-digest-dev
docker-get-kyverno-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

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
# Create e2e Infrastructure
##################################

.PHONY: kind-e2e-cluster
kind-e2e-cluster: $(KIND) ## Create kind cluster for e2e tests
	$(KIND) create cluster --image=$(KIND_IMAGE)

# TODO(eddycharly): $(REPO) is wrong, it is always ghcr.io/kyverno in the source
.PHONY: e2e-kustomize
e2e-kustomize: $(KUSTOMIZE) ## Build kustomize manifests for e2e tests
	cd config && \
	$(KUSTOMIZE) edit set image $(REPO)/$(KYVERNOPRE_IMAGE)=$(INITC_KIND_IMAGE):$(IMAGE_TAG_DEV) && \
	$(KUSTOMIZE) edit set image $(REPO)/$(KYVERNO_IMAGE)=$(KYVERNO_KIND_IMAGE):$(IMAGE_TAG_DEV)
	$(KUSTOMIZE) build config/ -o config/install.yaml

.PHONY: e2e-init-container
e2e-init-container: kind-e2e-cluster | ko-build-kyvernopre
	$(KIND) load docker-image $(INITC_KIND_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: e2e-kyverno-container
e2e-kyverno-container: kind-e2e-cluster | ko-build-kyverno
	$(KIND) load docker-image $(KYVERNO_KIND_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: create-e2e-infrastructure
create-e2e-infrastructure: e2e-init-container e2e-kyverno-container e2e-kustomize | ## Setup infrastructure for e2e tests

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
kind-deploy: $(KIND) ko-build-initContainer-local ko-build-kyverno-local
	$(KIND) load docker-image $(INITC_KIND_IMAGE):$(IMAGE_TAG_DEV)
	$(KIND) load docker-image $(KYVERNO_KIND_IMAGE):$(IMAGE_TAG_DEV)
	helm upgrade --install kyverno --namespace kyverno --wait --create-namespace ./charts/kyverno \
		--set image.repository=$(KYVERNO_KIND_IMAGE) \
		--set image.tag=$(IMAGE_TAG_DEV) \
		--set initImage.repository=$(INITC_KIND_IMAGE) \
		--set initImage.tag=$(IMAGE_TAG_DEV) \
		--set extraArgs={--autogenInternals=true}
	helm upgrade --install kyverno-policies --namespace kyverno --create-namespace ./charts/kyverno-policies
