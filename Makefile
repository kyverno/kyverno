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
REGISTRY=index.docker.io
REPO=$(REGISTRY)/nirmata/kyverno
IMAGE_TAG=$(GIT_VERSION)

GOOS ?= $(shell go env GOOS)
OUTPUT=$(shell pwd)/_output/cli/$(BIN)

build:
	GOOS=linux go build -ldflags=$(LD_FLAGS) $(MAIN)

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