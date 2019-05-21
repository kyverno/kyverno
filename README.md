# Kyverno - Kubernetes Native Policy Management

![logo](documentation/images/Kyverno_Horizontal.png)

Kyverno is a policy engine built for Kubernetes.

Kyverno policies are custom resources that are written in YAML or JSON. Kyverno policies can validate, mutate, and generate any Kubernetes resources. 

Kyverno runs as a [dynamic admission controller](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) in a Kubernetes cluster. Kyverno receives validating and mutating admission webhook HTTP callbacks from the kube-apiserver, applies matching polcies, and returns results that enforce admission policies or reject requests.

Policies match resources using the resource kind, name, and label selectors. Wildcards are supported in names.

Mutating policies can be written as overlays (similar to [Kustomize](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#bases-and-overlays)) or as a [JSON Patch](http://jsonpatch.com/). Validating policies also use an overlay style syntax, with support for pattern matching and conditional (if-then-else) processing. 

Policy enforcement is captured using Kubernetes events. Kyverno also reports policy violations for existing resources.

## Examples

### 1. Validating resources

### 2. Mutating resources

### 2. Generating resources

## Documentation

* [Getting Started](documentation/installation.md)
* [Writing Policies](documentation/writing-policies.md)
  * [Validate Rules](documentation/writing-policies.md)
  * [Mutate Rules](documentation/writing-policies.md)
  * [Generate Rules](documentation/writing-policies.md)
* [Testing Policies](documentation/testing-policies.md)

## Roadmap


## Getting help

* For feature requests and bugs, file an [issue][https://github.com/nirmata/kyverno/issues].
* For general discussion about both using and developing dex, join the [dex-dev][dex-dev] mailing list.

