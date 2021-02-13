.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################
GIT_VERSION := $(shell git describe --always --tags)
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

REGISTRY?=ghcr.io
REPO=$(REGISTRY)/kyverno
IMAGE_TAG?=$(GIT_VERSION)
GOOS ?= $(shell go env GOOS)
PACKAGE ?=github.com/kyverno/kyverno
LD_FLAGS="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"

##################################
# KYVERNO
##################################

KYVERNO_PATH:= cmd/kyverno
build: kyverno
PWD := $(CURDIR)

##################################
# INIT CONTAINER 
##################################
INITC_PATH := cmd/initContainer
INITC_IMAGE := kyvernopre
initContainer: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go

.PHONY: docker-build-initContainer docker-push-initContainer

docker-publish-initContainer: docker-build-initContainer docker-push-initContainer

docker-build-initContainer:
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-initContainer-amd64:
	@docker build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"

docker-push-initContainer:
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) .
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):latest .

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
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-publish-kyverno: docker-build-kyverno docker-push-kyverno

docker-build-kyverno:
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-kyverno-amd64:
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"

docker-push-kyverno:
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) .
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):latest .

##################################

# Generate Docs for types.go
##################################

generate-api-docs:
	go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir ./pkg/api -config documentation/api/config.json -template-dir documentation/api/template -out-file documentation/index.html


##################################
# CLI
##################################
.PHONY: docker-build-cli docker-push-cli
CLI_PATH := cmd/cli/kubectl-kyverno
KYVERNO_CLI_IMAGE := kyverno-cli

cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)/main.go

docker-publish-cli: docker-build-cli docker-push-cli

docker-build-cli:
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-cli-amd64:
	@docker build -f $(PWD)/$(CLI_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"

docker-push-cli:
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) .
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):latest .

##################################
docker-publish-all: docker-publish-initContainer docker-publish-kyverno docker-publish-cli

docker-build-all: docker-build-initContainer docker-build-kyverno docker-build-cli

docker-build-all-amd64: docker-build-initContainer-amd64 docker-build-kyverno-amd64 docker-build-cli-amd64

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
GO_ACC := $(BIN_DIR)/go-acc
CODE_COVERAGE_FILE:= coverage
CODE_COVERAGE_FILE_TXT := $(CODE_COVERAGE_FILE).txt
CODE_COVERAGE_FILE_HTML := $(CODE_COVERAGE_FILE).html

## targets
$(GO_ACC):
	@echo "	downloading testing tools"
	go get -v github.com/ory/go-acc
	$(eval export PATH=$(GO_ACC):$(PATH))
# go test provides code coverage per packages only.
# go-acc merges the result for pks so that it be used by	
# go tool cover for reporting

# go get downloads and installs the binary
# we temporarily add the GO_ACC to the path
test-all: $(GO_ACC)
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
	go test ./test/e2e/... -v
	$(eval export E2E="")

# godownloader create downloading script for kyverno-cli
godownloader:
	godownloader .goreleaser.yml --repo kyverno/kyverno -o ./scripts/install-cli.sh  --source="raw"

# kustomize-crd will create install.yaml 
kustomize-crd:
	# Create CRD for helm deployment Helm 
	kustomize build ./definitions/crds > ./charts/kyverno/crds/crds.yaml
	# Generate install.yaml that have all resources for kyverno
	kustomize build ./definitions > ./definitions/install.yaml
	# Generate install_debug.yaml that for developer testing
	kustomize build ./definitions/debug > ./definitions/install_debug.yaml

# guidance https://github.com/kyverno/kyverno/wiki/Generate-a-Release
release:
	kustomize build ./definitions > ./definitions/install.yaml
	kustomize build ./definitions > ./definitions/release/install.yaml

kyverno-crd: controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/api/kyverno/v1alpha1 output:dir=./definitions/crds
	$(CONTROLLER_GEN) crd paths=./pkg/api/kyverno/v1 output:dir=./definitions/crds

report-crd: controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/api/policyreport/v1alpha1 output:dir=./definitions/crds

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Run go fmt against code
fmt:
	gofmt -s -w .

vet:
	go vet ./...
