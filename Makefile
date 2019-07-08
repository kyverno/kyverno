.DEFAULT_GOAL: build


# The CLI binary to build
BIN ?= kyverno

GIT_VERSION := $(shell git describe --dirty --always --tags)
GIT_BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GIT_HASH := $(GIT_BRANCH)/$(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

PACKAGE ?=github.com/nirmata/kyverno
MAIN ?=$(PACKAGE)

LD_FLAGS="-s -w -X $(PACKAGE)/pkg/version.BuildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/version.BuildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/version.BuildTime=$(TIMESTAMP)"

# default docker hub
REGISTRY=registry-v2.nirmata.io
REPO=$(REGISTRY)/nirmata/kyverno
IMAGE_TAG=testImage

GOOS ?= $(shell go env GOOS)
OUTPUT=$(shell pwd)/_output/cli/$(BIN)

build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags=$(LD_FLAGS) $(MAIN)

local:
	go build -ldflags=$(LD_FLAGS) $(MAIN)

cli: cli-dirs
	GOOS=$(GOOS) \
    go build \
    -o $(OUTPUT) \
    -ldflags $(LD_FLAGS) \
    $(PACKAGE)/cmd/$(BIN)

cli-dirs:
	@mkdir -p _output/cli

clean:
	go clean

# docker image build targets
# user must be logged in the $(REGISTRY) to push images
.PHONY: docker-build docker-tag-repo docker-push

docker-publish: docker-build  docker-tag-repo  docker-push

docker-build:
	@docker build -t $(REPO):$(IMAGE_TAG) .

docker-tag-repo:
	@docker tag $(REPO):$(IMAGE_TAG) $(REPO):latest

docker-push:
	@docker push $(REPO):$(IMAGE_TAG)
	@docker push $(REPO):latest

## Testing & Code-Coverage

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