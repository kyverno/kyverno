.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################
GIT_VERSION := $(shell git describe --always --tags)
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

REGISTRY?=index.docker.io
REPO=$(REGISTRY)/nirmata/kyverno
IMAGE_TAG?=$(GIT_VERSION)
GOOS ?= $(shell go env GOOS)
PACKAGE ?=github.com/nirmata/kyverno
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

.PHONY: docker-build-initContainer docker-tag-repo-initContainer docker-push-initContainer

docker-publish-initContainer: docker-build-initContainer docker-tag-repo-initContainer docker-push-initContainer

docker-build-initContainer:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go
	echo $(PWD)/$(INITC_PATH)/
	@docker build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REGISTRY)/nirmata/$(INITC_IMAGE):$(IMAGE_TAG) $(PWD)/$(INITC_PATH)/

docker-tag-repo-initContainer:
	@docker tag $(REGISTRY)/nirmata/$(INITC_IMAGE):$(IMAGE_TAG) $(REGISTRY)/nirmata/$(INITC_IMAGE):latest

docker-push-initContainer:
	@docker push $(REGISTRY)/nirmata/$(INITC_IMAGE):$(IMAGE_TAG)
	@docker push $(REGISTRY)/nirmata/$(INITC_IMAGE):latest

##################################
# KYVERNO CONTAINER
##################################
.PHONY: docker-build-kyverno docker-tag-repo-kyverno docker-push-kyverno
KYVERNO_PATH := cmd/kyverno
KYVERNO_IMAGE := kyverno

local:
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)
	go build -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)

kyverno: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-publish-kyverno: docker-build-kyverno  docker-tag-repo-kyverno  docker-push-kyverno

docker-build-kyverno:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(PWD)/$(KYVERNO_PATH)

docker-tag-repo-kyverno:
	@echo "docker tag $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):latest"
	@docker tag $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):latest

docker-push-kyverno:
	@docker push $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG)
	@docker push $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):latest

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
docker-publish-all: docker-publish-initContainer docker-publish-kyverno

docker-build-all: docker-build-initContainer docker-build-kyverno

ci:
	echo "kustomize input"
	chmod a+x $(PWD)/scripts/ci.sh
	$(PWD)/scripts/ci.sh


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
	godownloader .goreleaser.yml --repo nirmata/kyverno -o ./scripts/install-cli.sh  --source="raw"

# kustomize-crd will create install.yaml 
kustomize-crd:
	# Create CRD for helm deployment Helm 
	kustomize build ./definitions/crds > ./charts/kyverno/crds/crds.yaml
	# Generate install.yaml that have all resources for kyverno
	kustomize build ./definitions > ./definitions/install.yaml
	# Generate install_debug.yaml that for developer testing
	kustomize build ./definitions/debug > ./definitions/install_debug.yaml

# guidance https://github.com/nirmata/kyverno/wiki/Generate-a-Release
release: 
	# update image tag
	cd ./definitions && kustomize edit set image nirmata/kyverno=nirmata/kyverno:$(IMAGE_TAG)
	cd ./definitions && kustomize edit set image nirmata/kyvernopre=nirmata/kyvernopre:$(IMAGE_TAG)

	kustomize build ./definitions > ./definitions/install.yaml
	kustomize build ./definitions > ./definitions/release/install.yaml

# Run go fmt against code
fmt:
	go fmt ./...

vet:
	go vet ./...
	
