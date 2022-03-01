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
CONTROLLER_GEN_REQ_VERSION := v0.4.0
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
# SIGNATURE CONTAINER
##################################
ALPINE_PATH := cmd/alpineBase
SIG_IMAGE := signatures
.PHONY: docker-build-signature docker-push-signature

docker-publish-sigs: docker-build-signature docker-push-signature

docker-build-signature:
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --tag $(REPO)/$(SIG_IMAGE):$(IMAGE_TAG) .

docker-push-signature:
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SIG_IMAGE):$(IMAGE_TAG) .
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SIG_IMAGE):latest .

##################################
# SBOM CONTAINER
##################################
ALPINE_PATH := cmd/alpineBase
SBOM_IMAGE := sbom
.PHONY: docker-build-sbom docker-push-sbom

docker-publish-sbom: docker-buildx-builder docker-build-sbom docker-push-sbom

docker-build-sbom: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --tag $(REPO)/$(SBOM_IMAGE):$(IMAGE_TAG) .

docker-push-sbom: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SBOM_IMAGE):$(IMAGE_TAG) .
	@docker buildx build --file $(PWD)/$(ALPINE_PATH)/Dockerfile --push --tag $(REPO)/$(SBOM_IMAGE):latest .
	
##################################
# INIT CONTAINER
##################################
INITC_PATH := cmd/initContainer
INITC_IMAGE := kyvernopre
initContainer: fmt vet
	GOOS=$(GOOS) go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go

.PHONY: docker-build-initContainer docker-push-initContainer

docker-buildx-builder:
	if ! docker buildx ls | grep -q kyverno; then\
		docker buildx create --name kyverno --use;\
	fi

docker-publish-initContainer: docker-buildx-builder docker-build-initContainer docker-push-initContainer

docker-build-initContainer: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-initContainer-amd64: 
	@docker build -f $(PWD)/$(INITC_PATH)/Dockerfile -t $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(INITC_IMAGE):latest

docker-push-initContainer: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-initContainer-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-build-initContainer-local:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(INITC_PATH)/kyvernopre -ldflags=$(LD_FLAGS) $(PWD)/$(INITC_PATH)/main.go
	@docker build -f $(PWD)/$(INITC_PATH)/localDockerfile -t $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(PWD)/$(INITC_PATH)
	@docker tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(INITC_IMAGE):latest

docker-publish-initContainer-dev: docker-buildx-builder docker-push-initContainer-dev

docker-push-initContainer-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(INITC_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(INITC_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS)

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
	GOOS=$(GOOS) go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(KYVERNO_PATH)/main.go

docker-publish-kyverno: docker-buildx-builder docker-build-kyverno docker-push-kyverno

docker-build-kyverno: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-kyverno-local:
	CGO_ENABLED=0 GOOS=linux go build -o $(PWD)/$(KYVERNO_PATH)/kyverno -ldflags=$(LD_FLAGS_DEV) $(PWD)/$(KYVERNO_PATH)/main.go
	@docker build -f $(PWD)/$(KYVERNO_PATH)/localDockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) -t $(REPO)/$(KYVERNO_IMAGE):latest $(PWD)/$(KYVERNO_PATH)
	@docker tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest

docker-build-kyverno-amd64:
	@docker build -f $(PWD)/$(KYVERNO_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_IMAGE):latest

docker-push-kyverno: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-kyverno-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-publish-kyverno-dev: docker-buildx-builder docker-push-kyverno-dev

docker-push-kyverno-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(KYVERNO_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-kyverno-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'
##################################

# Generate Docs for types.go
##################################

generate-api-docs:
	go run gen-crd-api-reference-docs -api-dir ./api -config docs/config.json -template-dir docs/template -out-file docs/crd/v1/index.html


##################################
# CLI
##################################
.PHONY: docker-build-cli docker-push-cli
CLI_PATH := cmd/cli/kubectl-kyverno
KYVERNO_CLI_IMAGE := kyverno-cli

cli:
	GOOS=$(GOOS) go build -o $(PWD)/$(CLI_PATH)/kyverno -ldflags=$(LD_FLAGS) $(PWD)/$(CLI_PATH)/main.go

docker-publish-cli: docker-buildx-builder docker-build-cli docker-push-cli

docker-build-cli: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-build-cli-amd64:
	@docker build -f $(PWD)/$(CLI_PATH)/Dockerfile -t $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS) --build-arg TARGETPLATFORM="linux/amd64"
	@docker tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) $(REPO)/$(KYVERNO_CLI_IMAGE):latest

docker-push-cli: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-cli-digest:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

docker-publish-cli-dev: docker-buildx-builder docker-push-cli-dev

docker-push-cli-dev: docker-buildx-builder
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_LATEST_DEV)-latest . --build-arg LD_FLAGS=$(LD_FLAGS_DEV)
	@docker buildx build --file $(PWD)/$(CLI_PATH)/Dockerfile --progress plane --push --platform linux/arm64,linux/amd64 --tag $(REPO)/$(KYVERNO_CLI_IMAGE):latest . --build-arg LD_FLAGS=$(LD_FLAGS)

docker-get-cli-digest-dev:
	@docker buildx imagetools inspect --raw $(REPO)/$(KYVERNO_CLI_IMAGE):$(IMAGE_TAG_DEV) | perl -pe 'chomp if eof' | openssl dgst -sha256 | sed 's/^.* //'

##################################
docker-publish-all: docker-buildx-builder docker-publish-initContainer docker-publish-kyverno docker-publish-cli

docker-build-all: docker-buildx-builder docker-build-initContainer docker-build-kyverno docker-build-cli

docker-build-all-amd64: docker-buildx-builder docker-build-initContainer-amd64 docker-build-kyverno-amd64 docker-build-cli-amd64

##################################
# Create e2e Infrastruture
##################################

create-e2e-infrastruture: docker-build-initContainer-local docker-build-kyverno-local
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

test: test-clean test-unit test-e2e

test-clean:
	@echo "	cleaning test cache"
	go clean -testcache ./...

.PHONY: test-cli
test-cli: test-cli-policies test-cli-local test-cli-local-mutate

.PHONY: test-cli-policies
test-cli-policies: cli
	cmd/cli/kubectl-kyverno/kyverno test https://github.com/kyverno/policies/$(TEST_GIT_BRANCH)

.PHONY: test-cli-local
test-cli-local: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test

.PHONY: test-cli-local-mutate
test-cli-local-mutate: cli
	cmd/cli/kubectl-kyverno/kyverno test ./test/cli/test


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

helm-test-values:
	sed -i -e "s|nameOverride:.*|nameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|fullnameOverride:.*|fullnameOverride: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|namespace:.*|namespace: kyverno|g" charts/kyverno/values.yaml
	sed -i -e "s|tag:  # replaced in e2e tests.*|tag: $(GIT_VERSION_DEV)|" charts/kyverno/values.yaml

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

