.DEFAULT_GOAL: build


# The CLI binary to build
BIN ?= kyverno

GIT_VERSION := $(shell git describe --dirty --always --tags)
GIT_HASH := $(shell git log -1 --pretty=format:"%H")
TIMESTAMP := $(shell date '+%Y-%m-%d_%I:%M:%S%p')

PACKAGE ?=github.com/nirmata/kyverno
MAIN ?=$(PACKAGE)

LD_FLAGS="-s -w -X $(PACKAGE)/pkg/kyverno/version.buildVersion=$(GIT_VERSION) -X $(PACKAGE)/pkg/kyverno/version.buildHash=$(GIT_HASH) -X $(PACKAGE)/pkg/kyverno/version.buildTime=$(TIMESTAMP)"

REPO=nirmata/kyverno
TAG=0.1

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

image:
	docker build -t $(REPO):$(TAG) .
	docker tag $(REPO):$(TAG) $(REPO):latest

push:
	docker push $(REPO):$(TAG)
	docker push $(REPO):latest

clean:
	go clean
