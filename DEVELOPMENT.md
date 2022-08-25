# Developer Instructions

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
