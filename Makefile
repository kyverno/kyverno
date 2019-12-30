.DEFAULT_GOAL: build

##################################
# DEFAULTS
##################################
REGISTRY=index.docker.io
REPO=$(REGISTRY)/nirmata/kyverno
IMAGE_TAG=$(GIT_VERSION)
GOOS ?= $(shell go env GOOS)
LD_FLAGS="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"

GIT_VERSION := $(shell git describe --dirty --always --tags)
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

##################################
# KYVERNO
##################################

KYVERNO_PATH:= cmd/kyverno
build: kyverno

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
kyverno:
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-publish-kyverno: docker-build-kyverno  docker-tag-repo-kyverno  docker-push-kyverno

docker-build-kyverno:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(PWD)/$(KYVERNO_PATH)

docker-tag-repo-kyverno:
	@docker tag $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG) $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):latest

docker-push-kyverno:
	@docker push $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):$(IMAGE_TAG)
	@docker push $(REGISTRY)/nirmata/$(KYVERNO_IMAGE):latest


##################################
# CLI
##################################
CLI_PATH := cmd/cli
cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyvernocli -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)/main.go


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