.DEFAULT_GOAL: kyverno

##################################
# DEFAULTS
##################################
PWD := $(CURDIR)
GIT_VERSION := $(shell git describe --match "v[0-9]*")
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')
# VERSION is used to be able to set the version during a release through an env variable.
VERSION ?= $(shell git describe --match "v[0-9]*")
# Docker related defaults
REGISTRY?=ghcr.io
REPO=$(REGISTRY)/kyverno
# Golang related defaults
GOOS ?= $(shell go env GOOS)
ifeq ($(GOOS), darwin)
SED=gsed
else
SED=sed
endif
PACKAGE ?=github.com/kyverno/kyverno
CONTROLLER_GEN=controller-gen
CONTROLLER_GEN_REQ_VERSION := v0.4.0
LD_FLAGS="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"
# Used to disable inclusion of cloud provider code in k8schain
# https://github.com/google/go-containerregistry/tree/main/pkg/authn/k8schain
TAGS=disable_aws,disable_azure,disable_gcp

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
# SIGNATURE CONTAINER
##################################
ALPINE_PATH := cmd/alpineBase
SIG_IMAGE := signatures
.PHONY: docker-build-signature docker-push-signature

docker-buildx-builder:
	if ! docker buildx ls | grep -q kyverno; then\
		docker buildx create --name kyverno --use;\
	fi

docker-publish-sigs: docker-buildx-builder docker-build-signature docker-push-signature

docker-build-signature: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --tag $(REPO)/$(SIG_IMAGE):$(GIT_VERSION) .

docker-push-signature: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SIG_IMAGE):$(GIT_VERSION) .
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SIG_IMAGE):latest .

##################################
# SBOM CONTAINER
##################################
ALPINE_PATH := cmd/alpineBase
SBOM_IMAGE := sbom
.PHONY: docker-build-sbom docker-push-sbom

docker-publish-sbom: docker-buildx-builder docker-build-sbom docker-push-sbom

docker-build-sbom: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --tag $(REPO)/$(SBOM_IMAGE):$(GIT_VERSION) .

docker-push-sbom: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SBOM_IMAGE):$(GIT_VERSION) .
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SBOM_IMAGE):latest .

##################################
# INIT CONTAINER
##################################
INITC_PATH := cmd/initContainer
INITC_IMAGE := kyvernopre

.PHONY: build-initContainer
build-initContainer: ## Build docker images for initContainer
	ARCH=amd64 $(MAKE) go-build-initContainer
	ARCH=arm64 $(MAKE) go-build-initContainer
	ARCH=linux/amd64,linux/arm64 $(MAKE) docker-build-initContainer

.PHONY: push-initContainer
push-initContainer: ## Build and push docker images for initContainer
	ARCH=arm64 $(MAKE) go-build-initContainer
	ARCH=amd64 $(MAKE) go-build-initContainer
	PLATFORM=linux/amd64,linux/arm64 $(MAKE) docker-push-initContainer

.PHONY: go-build-initContainer
go-build-initContainer:
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -o $(PWD)/$(INITC_PATH)/$(GOOS)/$(ARCH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go

.PHONY: docker-build-initContainer
docker-build-initContainer: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REPO)/$(INITC_IMAGE):$(GIT_VERSION) --platform "$(PLATFORM)" $(PWD)/$(INITC_PATH)

.PHONY: docker-push-initContainer
docker-push-initContainer: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(INITC_PATH)/Dockerfile --push -t $(REPO)/$(INITC_IMAGE):$(GIT_VERSION) -t $(REPO)/$(INITC_IMAGE):latest --platform "$(PLATFORM)" $(PWD)/$(INITC_PATH)

##################################
# KYVERNO CONTAINER
##################################
KYVERNO_PATH := cmd/kyverno
KYVERNO_IMAGE := kyverno

.PHONY: build-kyverno
build-kyverno: ## Build docker images for kyverno
	ARCH=amd64 $(MAKE) go-build-kyverno
	ARCH=arm64 $(MAKE) go-build-kyverno
	PLATFORM=linux/amd64,linux/arm64 $(MAKE) docker-build-kyverno

.PHONY: push-kyverno
push-kyverno: ## Build and push docker images for kyverno
	ARCH=amd64 $(MAKE) go-build-kyverno
	ARCH=arm64 $(MAKE) go-build-kyverno
	PLATFORM=linux/amd64,linux/arm64 $(MAKE) docker-push-kyverno

.PHONY: go-build-kyverno
go-build-kyverno:
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -o $(PWD)/$(KYVERNO_PATH)/$(GOOS)/$(ARCH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

.PHONY: docker-build-kyverno
docker-build-kyverno: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(GIT_VERSION) --platform "$(PLATFORM)" $(PWD)/$(KYVERNO_PATH)

.PHONY: docker-push-kyverno
docker-push-kyverno: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile --push -t $(REPO)/$(KYVERNO_IMAGE):$(GIT_VERSION) -t $(REPO)/$(KYVERNO_IMAGE):latest --platform "$(PLATFORM)" $(PWD)/$(KYVERNO_PATH)

kyverno: fmt vet
	GOOS=$(GOOS) CGO_ENABLED=0 go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -tags $(TAGS) -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-build-kyverno-local:
	CGO_ENABLED=0 GOOS=linux CGO_ENABLED=0 go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -tags $(TAGS) -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go
	@docker build -f $(PWD)/$(KYVERNO_PATH)/localDockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(GIT_VERSION) $(PWD)/$(KYVERNO_PATH)
	@docker tag $(REPO)/$(KYVERNO_IMAGE):$(GIT_VERSION) $(REPO)/$(KYVERNO_IMAGE):latest

##################################
# Generate Docs for types.go
##################################
generate-api-docs:
	go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir ./api -config documentation/api/config.json -template-dir documentation/api/template -out-file documentation/index.html

##################################
# CLI
##################################
CLI_PATH := cmd/cli/kubectl-kyverno
KYVERNO_CLI_IMAGE := kyverno-cli

.PHONY: build-cli
build-cli: ## Build docker images for the kyverno cli
	ARCH=amd64 $(MAKE) go-build-cli
	ARCH=arm64 $(MAKE) go-build-cli
	PLATFORM=linux/amd64,linux/arm64 $(MAKE) docker-build-cli

.PHONY: push-cli
push-cli: ## Build and push docker images for the kyverno cli
	ARCH=amd64 $(MAKE) go-build-cli
	ARCH=arm64 $(MAKE) go-build-cli
	PLATFORM=linux/amd64,linux/arm64 $(MAKE) docker-push-cli

.PHONY: go-build-cli
go-build-cli:
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -o $(PWD)/$(CLI_PATH)/$(GOOS)/$(ARCH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)/main.go

.PHONY: docker-build-cli
docker-build-cli: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(CLI_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_CLI_IMAGE):$(GIT_VERSION) --platform "$(PLATFORM)" $(PWD)/$(CLI_PATH)

.PHONY: docker-push-cli
docker-push-cli: docker-buildx-builder
	@docker buildx build -f $(PWD)/$(CLI_PATH)/Dockerfile --push -t $(REPO)/$(KYVERNO_CLI_IMAGE):$(GIT_VERSION) -t $(REPO)/$(KYVERNO_CLI_IMAGE):latest --platform "$(PLATFORM)" $(PWD)/$(CLI_PATH)

##################################
docker-publish-all: docker-buildx-builder docker-publish-initContainer docker-publish-kyverno docker-publish-cli

.PHONY: build-all-amd64
build-all-amd64:
	ARCH=amd64 $(MAKE) go-build-initContainer docker-build-initContainer
	ARCH=amd64 $(MAKE) go-build-kyverno docker-build-kyverno
	ARCH=amd64 $(MAKE) go-build-cli docker-build-cli

##################################
# Create e2e Infrastruture
##################################

create-e2e-infrastruture:
	chmod a+x $(PWD)/scripts/create-e2e-infrastruture.sh
	$(PWD)/scripts/create-e2e-infrastruture.sh


##################################

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

test: test-clean test-unit test-e2e test-cmd

test-clean:
	@echo "	cleaning test cache"
	go clean -testcache ./...


# go get downloads and installs the binary
# we temporarily add the GO_ACC to the path
test-unit: $(GO_ACC)
	@echo "	running unit tests"
	go-acc ./... -o $(CODE_COVERAGE_FILE_TXT)

code-cov-report: $(CODE_COVERAGE_FILE_TXT)
# transform to html format
	@echo "	generating code coverage report"
	go tool cover -html=coverage.txt
	if [ -a $(CODE_COVERAGE_FILE_HTML) ]; then open $(CODE_COVERAGE_FILE_HTML); fi;

# Test E2E
test-e2e:
	$(eval export E2E="ok")
	go test ./test/e2e/metrics -v
	go test ./test/e2e/mutate -v
	go test ./test/e2e/generate -v
	$(eval export E2E="")

test-e2e-local:
	$(eval export E2E="ok")
	kubectl apply -f https://raw.githubusercontent.com/kyverno/kyverno/main/config/github/rbac.yaml
	kubectl port-forward -n kyverno service/kyverno-svc-metrics  8000:8000 &
	go test ./test/e2e/metrics -v
	go test ./test/e2e/mutate -v
	go test ./test/e2e/generate -v
	kill  $!
	$(eval export E2E="")

#Test TestCmd Policy
test-cmd: go-build-cli
	$(PWD)/$(CLI_PATH)/kyverno test https://github.com/kyverno/policies/main
	$(PWD)/$(CLI_PATH)/kyverno test ./test/cli/test-mutate
	$(PWD)/$(CLI_PATH)/kyverno test ./test/cli/test
	$(PWD)/$(CLI_PATH)/kyverno test ./test/cli/test-fail/missing-policy && exit 1 || exit 0
	$(PWD)/$(CLI_PATH)/kyverno test ./test/cli/test-fail/missing-rule && exit 1 || exit 0
	$(PWD)/$(CLI_PATH)/kyverno test ./test/cli/test-fail/missing-resource && exit 1 || exit 0

# godownloader create downloading script for kyverno-cli
godownloader:
	godownloader .goreleaser.yml --repo kyverno/kyverno -o ./scripts/install-cli.sh  --source="raw"

# kustomize-crd will create install.yaml
kustomize-crd:
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

kyverno-crd: controller-gen
	$(CONTROLLER_GEN) crd paths=./api/kyverno/... crd:crdVersions=v1 output:dir=./config/crds

report-crd: controller-gen
	$(CONTROLLER_GEN) crd paths=./api/policyreport/... crd:crdVersions=v1 output:dir=./config/crds

# install the right version of controller-gen
install-controller-gen:
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_REQ_VERSION) ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
	CONTROLLER_GEN=$(GOPATH)/bin/controller-gen

# setup controller-gen with the right version, if necessary
controller-gen:
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

# Bootstrap auto-generable code associated with deepcopy
deepcopy-autogen: controller-gen
	$(CONTROLLER_GEN) object:headerFile="scripts/boilerplate.go.txt" paths="./..."

goimports:
ifeq (, $(shell which goimports))
	@{ \
	echo "goimports not found!";\
	echo "installing goimports...";\
	go get golang.org/x/tools/cmd/goimports;\
	}
else
GO_IMPORTS=$(shell which goimports)
endif

# Run go fmt against code
fmt: goimports
	go fmt ./... && $(GO_IMPORTS) -w ./

vet:
	go vet ./...
