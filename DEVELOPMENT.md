# Developer Instructions

This document covers basic needs to work with Kyverno codebase.

It contains instructions to build, run, and test Kyverno.

- [Tools](#tools)
- [Building and publishing an image locally](#building-and-publishing-an-image-locally)
- [Building local binaries](#building-local-binaries)
- [Building local images](#building-local-images)
    - [Building local images with docker](#building-local-images-with-docker)
    - [Building local images with ko](#building-local-images-with-ko)
- [Deploying a local build]

## Tools

Building and/or testing Kyverno requires additional tooling.

We use `make` to simplify installing the tools we use.

Tools will be installed in the `.tools` folder when possible, this allows keeping installed tools local to the Kyverno repository.
The `.tools` folder is ignored by `git` and binaries should not be committed.

> **Note**: If you don't install tools, they will be downloaded/installed as necessary when running `make` targets.

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

## Building local images

In the same spirit as [building local binaries](#building-local-binaries), you can build local docker images instead of local binaries.

Currently, we are supporting two build systems:
- [Building local images with docker](#building-local-images-with-docker)
- [Building local images with ko](#building-local-images-with-ko)

> **Note**: We started with `docker` and are progressively moving to `ko`.

As the `ko` based build system matures, we will deprecate and remove `docker` based builds.

Choosing between `docker` and `ko` boils down to a prefix when invoking `make` targets. 
For example:
- `make docker-build-kyverno` creates a docker image using the `docker` build system
- `make ko-build-kyverno` creates a docker image using the `ko` build system

<!-- TODO: explain the way images are tagged. -->

### Building local images with docker

When building local images with docker you can specify the registry used to create the image names by setting the `REGISTRY` environment variable (default value is `ghcr.io`).

> **Note**: You can build all local images at once by running `make docker-build-all`.

#### Building kyvernopre image locally

To build `kyvernopre` image on your local system, run:
```console
make docker-build-kyvernopre
```

The resulting image should be available locally, named `ghcr.io/kyverno/kyvernopre` (by default, if `REGISTRY` environment variable was not set).

#### Building kyverno image locally

To build `kyverno` image on your local system, run:
```console
make docker-build-kyverno
```

The resulting image should be available locally, named `ghcr.io/kyverno/kyverno` (by default, if `REGISTRY` environment variable was not set).

#### Building cli image locally

To build `cli` image on your local system, run:
```console
make docker-build-cli
```

The resulting image should be available locally, named `ghcr.io/kyverno/kyverno-cli` (by default, if `REGISTRY` environment variable was not set).

### Building local images with ko

When building local images with ko you can't specify the registry used to create the image names. It will always be `ko.local`.

> **Note**: You can build all local images at once by running `make ko-build-all`.

#### Building kyvernopre image locally

To build `kyvernopre` image on your local system, run:
```console
make ko-build-kyvernopre
```

The resulting image should be available locally, named `ko.local/github.com/kyverno/kyverno/cmd/initcontainer`.

#### Building kyverno image locally

To build `kyverno` image on your local system, run:
```console
make ko-build-kyverno
```

The resulting image should be available locally, named `ko.local/github.com/kyverno/kyverno/cmd/kyverno`.

#### Building cli image locally

To build `cli` image on your local system, run:
```console
make ko-build-cli
```

The resulting image should be available locally, named `ko.local/github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno`.

## Deploying a local build

After [building local images](#building-local-images), it is often usefull to deploy those images in a local cluster.

We use [KinD](https://kind.sigs.k8s.io/) to create local clusters easily.

### Create a local cluster

If you already have a local KinD cluster running, you can skip this step.

To create a local KinD cluster, run:
```console
make kind-create-cluster
```

You can override the k8s version by setting the `KIND_IMAGE` environment variable (default value is `kindest/node:v1.24.0`).

### Build and deploy local images

To build local images and deploy them on a local KinD cluster, run:
```console
# deploy kyverno helm chart
make kind-deploy-kyverno
```
or
```console
# deploy kyverno-policies helm chart
make kind-deploy-kyverno-policies
```
or
```console
# deploy both kyverno and kyverno-policies helm charts
make kind-deploy-all
```

This will build local images, load built images in every node of the KinD cluster, and deploy `kyverno` and/or `kyverno-policies` helm charts in the cluster (overriding image repositories and tags).

> **Note**: This actually uses `ko` to build local images.










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
