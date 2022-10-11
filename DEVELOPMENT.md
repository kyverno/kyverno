# Developer Instructions

This document covers basic needs to work with Kyverno codebase.

It contains instructions to build, run, and test Kyverno.

- [Tools](#tools)
- [Building local binaries](#building-local-binaries)
  - [Building kyvernopre locally](#building-kyvernopre-locally)
  - [Building kyverno locally](#building-kyverno-locally)
  - [Building cli locally](#building-cli-locally)
- [Building local images](#building-local-images)
  - [Building local images with docker](#building-local-images-with-docker)
  - [Building local images with ko](#building-local-images-with-ko)
  - [Switching between docker and ko](#switching-between-docker-and-ko)
- [Pushing images](#pushing-images)
  - [Pushing images with docker](#pushing-images-with-docker)
  - [Pushing images with ko](#pushing-images-with-ko)
- [Deploying a local build](#deploying-a-local-build)
  - [Create a local cluster](#create-a-local-cluster)
  - [Build and load local images](#build-and-load-local-images)
  - [Deploy with helm](#deploy-with-helm)
- [Code generation](#code-generation)
  - [Generating kubernetes API client](#generating-kubernetes-api-client)
  - [Generating API deep copy functions](#generating-api-deep-copy-functions)
  - [Generating CRD definitions](#generating-crd-definitions)
  - [Generating API docs](#generating-api-docs)
  - [Generating helm charts CRDs](#generating-helm-charts-crds)
  - [Generating helm charts docs](#generating-helm-charts-docs)
- [Debugging local code](#debugging-local-code)

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

It is also possible to [switch between docker and ko](#switching-between-docker-and-ko) build systems easily.

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

### Switching between docker and ko

The sections above cover building images with `docker` or `ko` by prefixing build commands (`docker-build-*` or `ko-build-*`).

You can achieve the same results by setting the `BUILD_WITH` environment variable, and invoke a generic `image-build-*` target:
```console
# build kyverno image with ko
BUILD_WITH=ko     make image-build-kyverno
# build kyverno image with docker
BUILD_WITH=docker make image-build-kyverno
```

Depending on the `BUILD_WITH` environment variable (default value is `ko`), the resulting images will be the same as noted in sections
[building local images with docker](#building-local-images-with-docker) and [building local images with ko](#building-local-images-with-ko).

## Pushing images

Pushing images is very similar to [building local images](#building-local-images), except that built images will be published on a remote image registry.

Currently, we are supporting two build systems:
- [Pushing images with docker](#pushing-images-with-docker)
- [Pushing images with ko](#pushing-images-with-ko)

> **Note**: We started with `docker` and are progressively moving to `ko`.

As the `ko` based build system matures, we will deprecate and remove `docker` based builds.

When pushing images you can specify the registry you want to publish images to by setting the `REGISTRY` environment variable (default value is `ghcr.io`).

<!-- TODO: explain the way images are tagged. -->

### Pushing images with docker

Authenticating to the remote registry is not done automatically in the `Makefile`.

You need to be authenticated before invoking targets responsible for pushing images.

> **Note**: You can push all images at once by running `make docker-publish-all` or `make docker-publish-all-dev`.

#### Pushing kyvernopre image

To push `kyvernopre` image on a remote registry, run:
```console
# push stable image
make docker-publish-kyvernopre
```
or
```console
# push dev image
make docker-publish-kyvernopre-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyvernopre` (by default, if `REGISTRY` environment variable was not set).

#### Pushing kyverno image

To push `kyverno` image on a remote registry, run:
```console
# push stable image
make docker-publish-kyverno
```
or
```console
# push dev image
make docker-publish-kyverno-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyverno` (by default, if `REGISTRY` environment variable was not set).

#### Pushing cli image

To push `cli` image on a remote registry, run:
```console
# push stable image
make docker-publish-cli
```
or
```console
# push dev image
make docker-publish-cli-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyverno-cli` (by default, if `REGISTRY` environment variable was not set).

### Pushing images with ko

Authenticating to the remote registry is done automatically in the `Makefile` with `ko login`.

To allow authentication you will need to set `REGISTRY_USERNAME` and `REGISTRY_PASSWORD` environment variables before invoking targets responsible for pushing images.

> **Note**: You can push all images at once by running `make ko-publish-all` or `make ko-publish-all-dev`.

#### Pushing kyvernopre image

To push `kyvernopre` image on a remote registry, run:
```console
# push stable image
make ko-publish-kyvernopre
```
or
```console
# push dev image
make ko-publish-kyvernopre-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyvernopre` (by default, if `REGISTRY` environment variable was not set).

#### Pushing kyverno image

To push `kyverno` image on a remote registry, run:
```console
# push stable image
make ko-publish-kyverno
```
or
```console
# push dev image
make ko-publish-kyverno-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyverno` (by default, if `REGISTRY` environment variable was not set).

#### Pushing cli image

To push `cli` image on a remote registry, run:
```console
# push stable image
make ko-publish-cli
```
or
```console
# push dev image
make ko-publish-cli-dev
```

The resulting image should be available remotely, named `ghcr.io/kyverno/kyverno-cli` (by default, if `REGISTRY` environment variable was not set).

## Deploying a local build

After [building local images](#building-local-images), it is often useful to deploy those images in a local cluster.

We use [KinD](https://kind.sigs.k8s.io/) to create local clusters easily, and have targets to:
- [Create a local cluster](#create-a-local-cluster)
- [Build and load local images](#build-and-load-local-images)
- [Deploy with helm](#deploy-with-helm)

### Create a local cluster

If you already have a local KinD cluster running, you can skip this step.

To create a local KinD cluster, run:
```console
make kind-create-cluster
```

You can override the k8s version by setting the `KIND_IMAGE` environment variable (default value is `kindest/node:v1.24.0`).

You can also override the KinD cluster name by setting the `KIND_NAME` environment variable (default value is `kind`).

### Build and load local images

To build local images and load them on a local KinD cluster, run:
```console
# build kyvernopre image and load it in KinD cluster
make kind-load-kyvernopre
```
or
```console
# build kyverno image and load it in KinD cluster
make kind-load-kyverno
```
or
```console
# build kyvernopre and kyverno images and load them in KinD cluster
make kind-load-all
```

You can override the KinD cluster name by setting the `KIND_NAME` environment variable (default value is `kind`).

In any case, you can choose the build system (`docker` or `ko`) by setting the `BUILD_WITH` environment variable:
> **Note**: See [switching between docker and ko](#switching-between-docker-and-ko).
```console
# build kyvernopre and kyverno images and load them in KinD cluster (with docker)
BUILD_WITH=docker make kind-load-all
```

### Deploy with helm

To build local images, load them on a local KinD cluster, and deploy helm charts, run:
```console
# build images, load them in KinD cluster and deploy kyverno helm chart
make kind-deploy-kyverno
```
or
```console
# deploy kyverno-policies helm chart
make kind-deploy-kyverno-policies
```
or
```console
# build images, load them in KinD cluster and deploy helm charts
make kind-deploy-all
```

This will build local images, load built images in every node of the KinD cluster, and deploy `kyverno` and/or `kyverno-policies` helm charts in the cluster (overriding image repositories and tags).

You can override the KinD cluster name by setting the `KIND_NAME` environment variable (default value is `kind`).

In any case, you can choose the build system (`docker` or `ko`) by setting the `BUILD_WITH` environment variable:
> **Note**: See [switching between docker and ko](#switching-between-docker-and-ko).
```console
# build images, load them in KinD cluster and deploy helm charts (with docker)
BUILD_WITH=docker make kind-deploy-all
```

## Code generation

We are using code generation tools to create the following portions of code:
- [Generating kubernetes API client](#generating-kubernetes-api-client)
- [Generating API deep copy functions](#generating-api-deep-copy-functions)
- [Generating CRD definitions](#generating-crd-definitions)
- [Generating API docs](#generating-api-docs)

> **Note**: You can run `make codegen-all` to build all generated code at once.

### Generating kubernetes API client

Based on the [APIs golang code definitions](./api), you can generate the corresponding Kubernetes client by running:
```console
# generate clientset, listers and informers
make codegen-client-all
```
or
```console
# generate clientset
make codegen-client-clientset
```
or
```console
# generate listers
make codegen-client-listers
```
or
```console
# generate informers
make codegen-client-informers
```

This will output generated files in the [/pkg/client](./pkg/client) package.

### Generating API deep copy functions

Based on the [APIs golang code definitions](./api), you can generate the corresponding deep copy functions by running:
```console
# generate all deep copy functions
make codegen-deepcopy-all
```
or
```console
# generate kyverno deep copy functions
make codegen-deepcopy-kyverno
```
or
```console
# generate policy reports deep copy functions
make codegen-deepcopy-report
```

This will output files named `zz_generated.deepcopy.go` in every API package.

### Generating CRD definitions

Based on the [APIs golang code definitions](./api), you can generate the corresponding CRDs manifests by running:
```console
# generate all CRDs
make codegen-crds-all
```
or
```console
# generate Kyverno CRDs
make codegen-crds-kyverno
```
or
```console
# generate policy reports CRDs
make codegen-crds-report
```

This will output CRDs manifests [/config/crds](./config/crds).

### Generating API docs

Based on the [APIs golang code definitions](./api), you can generate the corresponding API reference docs by running:
```console
# generate API docs
make codegen-api-docs
```

This will output API docs in [/docs/crd](./docs/crd).

### Generating helm charts CRDs

Based on the [APIs golang code definitions](./api), you can generate the corresponding CRD definitions for helm charts by running:
```console
# generate helm CRDs
make codegen-helm-crds
```

This will output CRDs templates in [/charts/kyverno/templates/crds.yaml](./charts/kyverno/templates/crds.yaml).

> **Note**: You can run `make codegen-helm-all` to generate CRDs and docs at once.

### Generating helm charts docs

Based on the helm charts default values:
- [kyverno](./charts/kyverno/values.yaml)
- [kyverno-policies](./charts/kyverno-policies/values.yaml)

You can generate the corresponding helm chart docs by running:
```console
# generate helm docs
make codegen-helm-docs
```

This will output docs in helm charts respective `README.md`:
- [kyverno](./charts/kyverno/README.md)
- [kyverno-policies](./charts/kyverno-policies/README.md)

> **Note**: You can run `make codegen-helm-all` to generate CRDs and docs at once.

## Debugging local code

Running Kyverno on a local machine without deploying it in a remote cluster can be useful, especially for debugging purpose.
You can run Kyverno locally or in your IDE of choice with a few steps:

1. Create a local cluster
    - You can create a simple cluster with [KinD](https://kind.sigs.k8s.io/) with `make kind-create-cluster`
1. Deploy Kyverno manifests except the Kyverno `Deployment`
    - Kyverno is going to run on your local machine so it should not run in cluster at the same time
    - You can deploy the manifests by running `make debug-deploy`
1. To run Kyverno locally against the remote cluster you will need to provide `--kubeconfig` and `--serverIP` arguments:
    - `--kubeconfig` must point to your kubeconfig file (usually `~/.kube/config`)
    - `--serverIP` must be set to `<local ip>:9443` (`<local ip>` is the private ip adress of your local machine)

Once you are ready with the steps above, Kyverno can be started locally with:
```console
go run ./cmd/kyverno/ --kubeconfig ~/.kube/config --serverIP=<local-ip>:9443
```

You will need to adapt those steps to run debug sessions in your IDE of choice, but the general idea remains the same.
