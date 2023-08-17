# Developer Instructions

This document covers basic needs to work with Kyverno codebase.

It contains instructions to build, run, and test Kyverno.

- [Open project in devcontainer](#open-project-in-devcontainer-recommended)
- [Tools](#tools)
- [Building local binaries](#building-local-binaries)
  - [Building kyvernopre locally](#building-kyvernopre-locally)
  - [Building kyverno locally](#building-kyverno-locally)
  - [Building cli locally](#building-cli-locally)
- [Building local images](#building-local-images)
  - [Building local images with ko](#building-local-images-with-ko)
- [Pushing images](#pushing-images)
  - [Images tagging strategy](#images-tagging-strategy)
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

## Open project in devcontainer (recommended)
- Clone the project to your local machine.
- Make sure that you have the Visual Studio Code editor installed on your system.

- Make sure that you have wsl(Ubuntu preferred) and Docker installed on your system and on wsl too (docker.sock (UNIX socket) file is necessary to enable devcontainer to communicate with docker running in host machine).

- Open the project in Visual Studio Code, once the project is opened hit F1 and type wsl, now click on "Reopen in WSL".

- If you haven't already done so, install the **Dev Containers** extension in Visual Studio Code.

- Once the extension is installed, you should see a green icon in the bottom left corner of the window.

- After you have installed Dev Containers extension, it should automatically detect the .devcontainer folder inside the project opened in wsl, and should suggest you to open the project in container.

- If it doesn't suggest you, then press <kbd>Ctrl</kbd> + <kbd>Shift</kbd> + <kbd>p</kbd> and search "reopen in container" and click on it.

- If everything goes well, the project should be opened in your devcontainer.

- Then follow the steps as mentioned below to configure the project.

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
make build-kyverno-init
```

The binary should be created at `./cmd/kyverno-init/kyvernopre`.

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

`ko` is used to build images, please refer to [Building local images with ko](#building-local-images-with-ko).

### Image tags

Building images uses repository tags. To fetch repository tags into your fork run the following commands:

```sh
git remote add upstream  https://github.com/kyverno/kyverno
git fetch upstream --tags
```

### Building local images with ko

When building local images with ko you can't specify the registry used to create the image names. It will always be `ko.local`.

> **Note**: You can build all local images at once by running `make ko-build-all`.

#### Building kyvernopre image locally

To build `kyvernopre` image on your local system, run:
```console
make ko-build-kyverno-init
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

## Pushing images

Pushing images is very similar to [building local images](#building-local-images), except that built images will be published on a remote image registry.

`ko` is used to build and publish images, please refer to [Pushing images with ko](#pushing-images-with-ko).

When pushing images you can specify the registry you want to publish images to by setting the `REGISTRY` environment variable (default value is `ghcr.io`).

### Images tagging strategy

When publishing images, we are using the following strategy:
- All published images are tagged with `latest`. Images tagged with `latest` should not be considered stable and can come from multiple release branches or main.
- In addition to `latest`, dev images are tagged with the following pattern `<major>.<minor>-dev-N-<git hash>` where `N` is a two-digit number beginning at one for the major-minor combination and incremented by one on each subsequent tagged image.
- In addition to `latest`, release images are tagged with the following pattern `<major>.<minor>.<patch>-<pre release>`. The pre release part is optional and only applies to pre releases (`-beta.1`, `-rc.2`, ...).

### Pushing images with ko

Authenticating to the remote registry is done automatically in the `Makefile` with `ko login`.

To allow authentication you will need to set `REGISTRY_USERNAME` and `REGISTRY_PASSWORD` environment variables before invoking targets responsible for pushing images.

> **Note**: You can push all images at once by running `make ko-publish-all` or `make ko-publish-all-dev`.

#### Pushing kyvernopre image

To push `kyvernopre` image on a remote registry, run:
```console
# push stable image
make ko-publish-kyverno-init
```
or
```console
# push dev image
make ko-publish-kyverno-init-dev
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
make kind-load-kyverno-init
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
    - Kyverno is going to run on your local machine, so it should not run in cluster at the same time
    - You can deploy the manifests by running `make debug-deploy`
1. There are multiple environment variables that need to be configured. The variables can be found in [here](./.vscode/launch.json). Their values can be set using the command `export $NAME=value`
1. To run Kyverno locally against the remote cluster you will need to provide `--kubeconfig` and `--serverIP` arguments:
    - `--kubeconfig` must point to your kubeconfig file (usually `~/.kube/config`)
    - `--serverIP` must be set to `<local ip>:9443` (`<local ip>` is the private ip adress of your local machine)
    - `--backgroundServiceAccountName` must be set to `system:serviceaccount:kyverno:kyverno-background-controller`

Once you are ready with the steps above, Kyverno can be started locally with:
```console
go run ./cmd/kyverno/ --kubeconfig ~/.kube/config --serverIP=<local-ip>:9443 --backgroundServiceAccountName=system:serviceaccount:kyverno:kyverno-background-controller
```

You will need to adapt those steps to run debug sessions in your IDE of choice, but the general idea remains the same.
