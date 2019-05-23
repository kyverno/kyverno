.DEFAULT_GOAL: build
PACKAGE ?=github.com/nirmata/kyverno
MAIN ?=$(PACKAGE)
LD_FLAGS ="-s -w"

REPO=nirmata/kyverno
TAG=0.1

build:
	GOOS=linux go build -ldflags=$(LD_FLAGS) $(MAIN)

local:
	go build -ldflags=$(LD_FLAGS) $(MAIN)

image:
	docker build -t $(REPO):$(TAG) .
	docker tag $(REPO):$(TAG) $(REPO):latest

push:
	docker push $(REPO):$(TAG)
	docker push $(REPO):latest

clean:
	go clean
