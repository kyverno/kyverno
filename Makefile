.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################
GIT_VERSION := $(shell git describe --dirty --always --tags)
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
GIT_SHORT_HASH := $(shell git rev-parse --short HEAD)
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

REGISTRY?=index.docker.io
REPO=$(REGISTRY)/evalsocket/kyverno

IMAGE_TAG?=$(GIT_VERSION)
GOOS ?= $(shell go env GOOS)
PACKAGE ?=github.com/evalsocket/kyverno
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
initContainer:
	GOOS=$(GOOS) go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go

.PHONY: docker-build-initContainer docker-tag-repo-initContainer docker-push-initContainer

docker-publish-initContainer: docker-build-initContainer docker-tag-repo-initContainer docker-push-initContainer

docker-build-initContainer:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go
	echo $(PWD)/$(INITC_PATH)/
	@docker build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REGISTRY)/evalsocket/$(INITC_IMAGE):$(IMAGE_TAG) $(PWD)/$(INITC_PATH)/

docker-tag-repo-initContainer:
	@docker tag $(REGISTRY)/evalsocket/$(INITC_IMAGE):$(IMAGE_TAG) $(REGISTRY)/evalsocket/$(INITC_IMAGE):latest

docker-push-initContainer:
	@docker push $(REGISTRY)/evalsocket/$(INITC_IMAGE):$(IMAGE_TAG)
	@docker push $(REGISTRY)/evalsocket/$(INITC_IMAGE):latest

##################################
# KYVERNO CONTAINER
##################################
.PHONY: docker-build-kyverno docker-tag-repo-kyverno docker-push-kyverno
KYVERNO_PATH := cmd/kyverno
KYVERNO_IMAGE := kyverno

local:
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)

kyverno:
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-publish-kyverno: docker-build-kyverno  docker-tag-repo-kyverno  docker-push-kyverno

docker-build-kyverno:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REGISTRY)/evalsocket/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(PWD)/$(KYVERNO_PATH)

docker-tag-repo-kyverno:
	@docker tag $(REGISTRY)/evalsocket/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(REGISTRY)/evalsocket/$(KYVERNO_IMAGE):latest

docker-push-kyverno:
	@docker push $(REGISTRY)/evalsocket/$(KYVERNO_IMAGE):$(IMAGE_TAG)
	@docker push $(REGISTRY)/evalsocket/$(KYVERNO_IMAGE):latest

##################################
ci: docker-build-kyverno docker-build-initContainer
	echo "kustomize input"
	$(PWD)/scripts/ci.sh



##################################
# Generate Docs for types.go
##################################

generate-api-docs:
	go run github.com/ahmetb/gen-crd-api-reference-docs -api-dir ./pkg/api -config documentation/api/config.json -template-dir documentation/api/template -out-file documentation/index.html


##################################
# CLI
##################################
CLI_PATH := cmd/cli/kubectl-kyverno
cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)/main.go


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

# godownloader create downloading script for kyverno-cli
godownloader:
	godownloader .goreleaser.yml --repo nirmata/kyverno -o ./scripts/install-cli.sh  --source="raw"

# kustomize-crd will create install.yaml 
kustomize-crd:
	# Create CRD for helm deployment Helm 
	kustomize build ./definitions/crds > ./charts/kyverno/crds/crds.yaml
	# Generate install.yaml that have all resources for kyverno
	kustomize build ./definitions > ./definitions/install.yaml
	# Generate install_debug.yaml that for developer testing
	kustomize build ./definitions/debug > ./definitions/install_debug.yaml