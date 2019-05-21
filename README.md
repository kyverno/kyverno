# Kyverno - Kubernetes Native Policy Management

![logo](documentation/images/Kyverno_Horizontal.png)

Kyverno is a policy engine built for Kubernetes.

Kyverno policies are Kubernetes custom resources that can be written in YAML or JSON. Kyverno policies can validate, mutate, and generate any Kubernetes resources. 

Kyverno runs as a [dynamic admission controller](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) in a Kubernetes cluster. Kyverno receives validating and mutating admission webhook HTTP callbacks from the kube-apiserver and applies matching polcies to return results that enforce admission policies or reject requests.

Kyverno policies can match resources using the resource kind, name, and label selectors. Wildcards are supported in names.

Mutating policies can be written as overlays (similar to [Kustomize](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#bases-and-overlays)) or as a [JSON Patch](http://jsonpatch.com/). Validating policies also use an overlay style syntax, with support for pattern matching and conditional (if-then-else) processing. 

Policy enforcement is captured using Kubernetes events. Kyverno also reports policy violations for existing resources.

## Status

*Kyverno is under active development and not ready for production use.  Key components and policy definitions are likely to change as we complete core features.*

## Examples

### 1. Validating resources

This policy requires that all pods have CPU and memory resource requests and limits:

````yaml
apiVersion: policy.nirmata.io/v1alpha1
kind: Policy
metadata:
  name: check-cpu-memory
spec:
  rules:
  - name: check-pod-resources
    resource:
      kind: Pod
    validate:
      message: "CPU and memory resource requests and limits are required"
      pattern:
      spec:
        containers:
        - name: "*"
          resources:
            limits:
              memory: "?"
              cpu: "?"
            requests:
              memory: "?"
              cpu: "?"
````

### 2. Mutating resources

This policy sets the imagePullPolicy to Always if the image tag is latest:

````yaml
apiVersion: policy.nirmata.io/v1alpha1
kind: Policy
metadata:
  name: set-image-pull-policy
spec:
  rules:
  - name: set-image-pull-policy
    resource:
      kind: Pod
    mutate:
      overlay:
        spec:
          containers:
            # match images which end with :latest   
            - image: "(*:latest)"
              # set the imagePullPolicy to "Always"
              imagePullPolicy: "Always"
````

### 3. Generating resources

This policy sets the Zookeeper and Kafka connection strings for all namespaces with a label key 'kafka'.

````yaml
apiVersion: policy.nirmata.io/v1alpha1
kind: Policy
metadata:
  name: "zk-kafka-address"
spec:
  rules:
  - name: "zk-kafka-address"
    resource:
      kind : Namespace
      selector:
        matchExpressions:
        - {key: kafka, operator: Exists}
    generate:
      kind: ConfigMap
      name: zk-kafka-address
      data:
        ZK_ADDRESS: "192.168.10.10:2181,192.168.10.11:2181,192.168.10.12:2181"
        KAFKA_ADDRESS: "192.168.10.13:9092,192.168.10.14:9092,192.168.10.15:9092"
````

### 4. More examples

Additional examples are available in [examples](/examples).


## Documentation

* [Getting Started](documentation/installation.md)
* [Writing Policies](documentation/writing-policies.md)
  * [Validate Rules](documentation/writing-policies.md)
  * [Mutate Rules](documentation/writing-policies.md)
  * [Generate Rules](documentation/writing-policies.md)
* [Testing Policies](documentation/testing-policies.md)


## Status and Roadmap

Here are some the major features we plan on completing before a 1.0 release:

* Events
* Policy Violations
* Generate any resource
* Conditionals on existing resources
* Extend CLI to operate on cluster resources 

## Getting help

For feature requests and bugs, file an [issue][https://github.com/nirmata/kube-policy/issues].

