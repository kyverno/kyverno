.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################

GIT_VERSION := $(shell git describe --match "v[0-9]*" --tags $(git rev-list --tags --max-count=1))
GIT_VERSION_DEV := $(shell git describe --match "[0-9].[0-9]-dev*")
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')
CONTROLLER_GEN=controller-gen
CONTROLLER_GEN_REQ_VERSION := v0.8.0
VERSION ?= $(shell git describe --match "v[0-9]*")

REGISTRY?=ghcr.io
REPO=$(REGISTRY)/kyverno
IMAGE_TAG_LATEST_DEV=$(shell git describe --match "[0-9].[0-9]-dev*" | cut -d '-' -f-2)
IMAGE_TAG_DEV=$(GIT_VERSION_DEV)
IMAGE_TAG?=$(GIT_VERSION)
GOOS ?= $(shell go env GOOS)
ifeq ($(GOOS), darwin)
SED=gsed
else
SED=sed
endif
PACKAGE ?=github.com/kyverno/kyverno
LD_FLAGS="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"
LD_FLAGS_DEV="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION_DEV) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"
K8S_VERSION ?= $(shell kubectl version --short | grep -i server | cut -d" " -f3 | cut -c2-)
export K8S_VERSION
TEST_GIT_BRANCH ?= main

KIND_VERSION=v0.14.0
KIND_IMAGE?=kindest/node:v1.24.0

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
	GOOS=$(GOOS) go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)

.PHONY: docker-build-initContainer docker-push-initContainer

docker-buildx-builder:
	if ! docker buildx ls | grep -q kyverno; then\
		docker buildx create --name kyverno --use;\
	fi

docker-publish-initContainer: docker-buildx-builder docker-build-initContainer docker-push-initContainer

docker-build-initContainer: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-initContainer-amd64: 
	@docker build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(INITC_IMAGE):latest

docker-push-initContainer: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-initContainer-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-build-initContainer-local:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS_DEV) $(PWD)/$(INITC_PATH)
	@docker build -f $(PWD)/$(INITC_PATH)/localDockerfile -t $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(PWD)/$(INITC_PATH)
	@docker tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(INITC_IMAGE):latest

docker-publish-initContainer-dev: docker-buildx-builder docker-push-initContainer-dev

docker-push-initContainer-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(INITC_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

docker-get-initContainer-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

##################################
# KYVERNO CONTAINER
##################################

.PHONY: docker-build-kyverno docker-push-kyverno
KYVERNO_PATH := cmd/kyverno
KYVERNO_IMAGE := kyverno

local:
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)

kyverno: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)

docker-publish-kyverno: docker-buildx-builder docker-build-kyverno docker-push-kyverno

docker-build-kyverno: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-kyverno-local:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS_DEV) $(PWD)/$(KYVERNO_PATH)
	@docker build -f $(PWD)/$(KYVERNO_PATH)/localDockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) -t $(REPO)/$(KYVERNO_IMAGE):latest $(PWD)/$(KYVERNO_PATH)
	@docker tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest

docker-build-kyverno-amd64:
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_IMAGE):latest

docker-push-kyverno: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-kyverno-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-publish-kyverno-dev: docker-buildx-builder docker-push-kyverno-dev

docker-push-kyverno-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

docker-get-kyverno-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

##################################
# Generate Docs for types.go
##################################

.PHONY: gen-crd-api-reference-docs
gen-crd-api-reference-docs: ## Install gen-crd-api-reference-docs
	go install github.com/ahmetb/gen-crd-api-reference-docs@latest

.PHONY: gen-crd-api-reference-docs
generate-api-docs: gen-crd-api-reference-docs ## Generate api reference docs
	rm -rf docs/crd
	mkdir docs/crd
	gen-crd-api-reference-docs -v 6 -api-dir ./api/kyverno/v1alpha2 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1alpha2/index.html
	gen-crd-api-reference-docs -v 6 -api-dir ./api/kyverno/v1beta1 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1beta1/index.html
	gen-crd-api-reference-docs -v 6 -api-dir ./api/kyverno/v1 -config docs/config.json -template-dir docs/template -out-file docs/crd/v1/index.html

.PHONY: verify-api-docs
verify-api-docs: generate-api-docs ## Check api reference docs are up to date
	git --no-pager diff docs
	@echo 'If this test fails, it is because the git diff is non-empty after running "make generate-api-docs".'
	@echo 'To correct this, locally run "make generate-api-docs", commit the changes, and re-run tests.'
	git diff --quiet --exit-code docs

##################################
# CLI
##################################
.PHONY: docker-build-cli docker-push-cli
CLI_PATH := cmd/cli/kubectl-kyverno
KYVERNO_CLI_IMAGE := kyverno-cli

cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)

docker-publish-cli: docker-buildx-builder docker-build-cli docker-push-cli

docker-build-cli: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-cli-amd64:
	@docker build -f $(PWD)/$(CLI_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_CLI_IMAGE):latest

docker-push-cli: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-cli-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-publish-cli-dev: docker-buildx-builder docker-push-cli-dev

docker-push-cli-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64,linux/s390x --tag $(REPO)/$(KYVERNO_CLI_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)

docker-get-cli-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

##################################
docker-publish-all: docker-buildx-builder docker-publish-initContainer docker-publish-kyverno docker-publish-cli

docker-build-all: docker-buildx-builder docker-build-initContainer docker-build-kyverno docker-build-cli

docker-build-all-amd64: docker-buildx-builder docker-build-initContainer-amd64 docker-build-kyverno-amd64 docker-build-cli-amd64

##################################
# Create e2e Infrastruture
##################################

.PHONY: kind-install
kind-install: ## Install kind
ifeq (, $(shell which kind))
	go install sigs.k8s.io/kind@$(KIND_VERSION)
endif

.PHONY: kind-e2e-cluster
kind-e2e-cluster: kind-install ## Create kind cluster for e2e tests
	kind create cluster --image=$(KIND_IMAGE)

.PHONY: e2e-kustomize
e2e-kustomize: kustomize ## Build kustomize manifests for e2e tests
	cd config && \
	kustomize edit set image $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) && \
	kustomize edit set image $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV)
	kustomize build config/ -o config/install.yaml

.PHONY: e2e-init-container
e2e-init-container: kind-e2e-cluster docker-build-initContainer-local
	kind load docker-image $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: e2e-kyverno-container
e2e-kyverno-container: kind-e2e-cluster docker-build-kyverno-local
	kind load docker-image $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV)

.PHONY: create-e2e-infrastruture
create-e2e-infrastruture: e2e-init-container e2e-kyverno-container e2e-kustomize ## Setup infrastructure for e2e tests

##################################
# Testing & Code-Coverage
##################################

## variables
BIN_DIR := $(GOPATH)/bin
GO_ACC := $(BIN_DIR)/go-acc@latest
CODE_COVERAGE_FILE:= coverage
CODE_COVERAGE_FILE_TXT := $(CODE_COVERAGE_FILE).txt
CODE_COVERAGE_FILE_HTML := $(CODE_COVERAGE_FILE).html

## targets
$(GO_ACC):
	@echo "	installing testing tools"
	go install -v github.com/ory/go-acc@latest
	$(eval export PATH=$(GO_ACC):$(PATH))
# go test provides code coverage per packages only.
# go-acc merges the result for pks so that it be used by
# go tool cover for reporting

test: test-clean test-unit test-e2e ## Clean tests cache then run unit and e2e tests

test-clean: ## Clean tests cache
	@echo "	cleaning test cache"
	go clean -testcache ./...

.PHONY: test-cli
test-cli: test-cli-policies test-cli-local test-cli-local-mutate test-cli-test-case-selector-flag test-cli-registry

.PHONY: test-cli-policies
test-cli-policies: cli
	cmd/cli/kubectl-kyverno/kyverno test https://github.com/kyverno/policies/$(TEST_GIT_BRANCH)

.PHONY: test-cli-local
test-cli-local: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test

.PHONY: test-cli-local-mutate
test-cli-local-mutate: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test-mutate

.PHONY: test-cli-test-case-selector-flag
test-cli-test-case-selector-flag: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"

.PHONY: test-cli-registry
test-cli-registry: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/registry --registry

test-unit: $(GO_ACC) ## Run unit tests
	@echo "	running unit tests"
	go-acc ./... -o $(CODE_COVERAGE_FILE_TXT)

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
	sed -i -e "s|tag:  # replaced in e2e tests.*|tag: $(GIT_VERSION_DEV)|" charts/kyverno/values.yaml

# godownloader create downloading script for kyverno-cli
godownloader:
	godownloader .goreleaser.yml --repo kyverno/kyverno -o ./scripts/install-cli.sh  --source="raw"

.PHONY: kustomize
kustomize: ## Install kustomize
ifeq (, $(shell which kustomize))
	go install sigs.k8s.io/kustomize/kustomize/v4@latest
endif

.PHONY: kustomize-crd
kustomize-crd: kustomize ## Create install.yaml
	# Create CRD for helm deployment Helm
	kustomize build ./config/release | kustomize cfg grep kind=CustomResourceDefinition | $(SED) -e "1i{{- if .Values.installCRDs }}" -e '$$a{{- end }}' > ./charts/kyverno/templates/crds.yaml
	# Generate install.yaml that have all resources for kyverno
	kustomize build ./config > ./config/install.yaml
	# Generate install_debug.yaml that for developer testing
	kustomize build ./config/debug > ./config/install_debug.yaml

# guidance https://github.com/kyverno/kyverno/wiki/Generate-a-Release
release:
	kustomize build ./config > ./config/install.yaml
	kustomize build ./config/release > ./config/release/install.yaml

release-notes:
	@bash -c 'while IFS= read -r line ; do if [[ "$$line" == "## "* && "$$line" != "## $(VERSION)" ]]; then break ; fi; echo "$$line"; done < "CHANGELOG.md"' \
	true

##################################
# CODEGEN
##################################

.PHONY: kyverno-crd
kyverno-crd: controller-gen ## Generate Kyverno CRDs
	$(CONTROLLER_GEN) crd paths=./api/kyverno/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: report-crd
report-crd: controller-gen ## Generate policy reports CRDs
	$(CONTROLLER_GEN) crd paths=./api/policyreport/... crd:crdVersions=v1 output:dir=./config/crds

.PHONY: install-controller-gen
install-controller-gen: ## Install controller-gen
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go mod edit -replace=sigs.k8s.io/controller-tools@$(CONTROLLER_GEN_REQ_VERSION)=github.com/eddycharly/controller-tools@704af868d45a3a78448b9a6a2279c12ea96a621e ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_REQ_VERSION) ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
	CONTROLLER_GEN=$(GOPATH)/bin/controller-gen

.PHONY: controller-gen
controller-gen: ## Setup controller-gen
ifeq (, $(shell which controller-gen))
	@{ \
	echo "controller-gen not found!";\
	echo "installing controller-gen $(CONTROLLER_GEN_REQ_VERSION)...";\
	make install-controller-gen;\
	}
else ifneq (Version: $(CONTROLLER_GEN_REQ_VERSION), $(shell controller-gen --version))
	@{ \
		echo "controller-gen $(shell controller-gen --version) found!";\
		echo "required controller-gen $(CONTROLLER_GEN_REQ_VERSION)";\
		echo "installing controller-gen $(CONTROLLER_GEN_REQ_VERSION)...";\
		make install-controller-gen;\
	}
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: deepcopy-autogen
deepcopy-autogen: controller-gen ## Generate deep copy code
	$(CONTROLLER_GEN) object:headerFile="scripts/boilerplate.go.txt" paths="./..."

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

.PHONY: goimports
goimports: ## Install goimports if needed
ifeq (, $(shell which goimports))
	@{ \
	echo "goimports not found!";\
	echo "installing goimports...";\
	go install golang.org/x/tools/cmd/goimports@latest;\
	}
else
GO_IMPORTS=$(shell which goimports)
endif

.PHONY: fmt
fmt: goimports ## Run go fmt
	go fmt ./... && $(GO_IMPORTS) -w ./

.PHONY: vet
vet: ## Run go vet
	go vet ./...

##################################
# HELM
##################################

.PHONY: gen-helm-docs
gen-helm-docs: ## Generate Helm docs
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
kind-deploy: docker-build-initContainer-local docker-build-kyverno-local
	kind load docker-image $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV)
	kind load docker-image $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV)
	helm upgrade --install kyverno --namespace kyverno --wait --create-namespace ./charts/kyverno \
		--set image.repository=$(REPO)/$(KYVERNO_IMAGE) \
		--set image.tag=$(IMAGE_TAG_DEV) \
		--set initImage.repository=$(REPO)/$(INITC_IMAGE) \
		--set initImage.tag=$(IMAGE_TAG_DEV) \
		--set extraArgs={--autogenInternals=false}
	helm upgrade --install kyverno-policies --namespace kyverno --create-namespace ./charts/kyverno-policies
