# Developer Instructions

This document covers basic needs to work with Kyverno codebase.

It contains instructions to build, run, and test Kyverno.

- [Tools](#tools)
- [Building and publishing an image locally](#building-and-publishing-an-image-locally)
- [Building local binaries](#building-local-binaries)

## Tools

Building and/or testing Kyverno requires additional tooling.

We use `make` to simplify installing the tools we use.

Tools will be installed in the `.tools` folder when possible, this allows keeping installed tools local to the Kyverno repository.
The `.tools` folder is ignored by `git` and binaries should not be commited.

> **Note**: If you don't install tools, they will be download/installed as necessary when running `make` targets.

You can manually install tools by running:
```console
make install-tools
```

To remove installed tools, run:
```console
make clean-tools
```

## Building local binaries

The Kyverno repository contains code for three different binaries:
- [`kyvernopre`](#building-kyvernopre-locally):
  Binary to update/cleanup existing resources in clusters. This is typically run as an init container before Kyverno controller starts.
- [`kyverno`](#building-kyverno-locally):
  The Kyverno controller binary.
- [`cli`](#building-cli-locally):
  The Kyverno command line interface.

> **Note**: You can build all binaries at once by running `make build-all`.

### Building kyvernopre locally

To build `kyvernopre` binary on your local system, run:
```console
make build-kyvernopre
```

The binary should be created at `./cmd/initContainer/kyvernopre`.

### Building kyverno locally

To build `kyverno` binary on your local system, run:
```console
make build-kyverno
```

The binary should be created at `./cmd/kyverno/kyverno`.

### Building cli locally

To build `cli` binary on your local system, run:
```console
make build-cli
```

The binary should be created at `./cmd/cli/kubectl-kyverno/kubectl-kyverno`.

## Building and publishing an image locally

First, make sure you [install `ko`](https://github.com/google/ko#install)

### Publishing to your local Docker daemon

Set the `KO_DOCKER_REPO` environment variable to `ko.local`:

```
KO_DOCKER_REPO=ko.local
```

Then build and publish an image:

```
ko build ./cmd/kyverno --preserve-import-paths
```

The image will be available locally as `ko.local/github.com/kyverno/kyverno/cmd/kyverno`.

### Publishing to a local [KinD](https://kind.sigs.k8s.io/) cluster

First, create your KinD cluster:

```
kind create cluster
```

Set the `KO_DOCKER_REPO` environment variable to `kind.local`:

```
KO_DOCKER_REPO=kind.local
```

Then build and publish an image:

```
ko build ./cmd/kyverno --preserve-import-paths
```

This will build and load the image into your KinD cluster as:

```
kind.local/github.com/kyverno/kyverno/cmd/kyverno
```

If you have multiple KinD clusters, or created them with a non-default name, set `KIND_CLUSTER_NAME=<your-cluster-name>`.

### Publishing to a remote registry

Set the `KO_DOCKER_REPO` environment variable to the registry you'd like to push to:
For example:

```
KO_DOCKER_REPO=gcr.io/my-project/kyverno
KO_DOCKER_REPO=my-dockerhub-user/my-dockerhub-repo
KO_DOCKER_REPO=<ACCOUNTID>.dkr.ecr.<REGION>.amazonaws.com
```

Then build and publish an image:

```
ko build ./cmd/kyverno
```

The output will tell you the image name and digest of the image you just built.
