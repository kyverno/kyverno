# Kyverno - Kubernetes Native Policy Management

[![Build Status](https://travis-ci.org/nirmata/kyverno.svg?branch=master)](https://travis-ci.org/nirmata/kyverno) [![Go Report Card](https://goreportcard.com/badge/github.com/nirmata/kyverno)](https://goreportcard.com/report/github.com/nirmata/kyverno)

![logo](documentation/images/Kyverno_Horizontal.png)

Kyverno is a policy engine designed for Kubernetes.

Kubernetes supports declarative validation, mutation, and generation of resource configurations using policies written as Kubernetes resources. 

Kyverno can be used to scan existing workloads for best practices, or can be used to enforce best practices by blocking or mutating API requests.Kyverno allows cluster adminstrators to manage environment specific configurations independently of workload configurations and enforce configuration best practices for their clusters.

Kyverno policies are Kubernetes resources that can be written in YAML or JSON. Kyverno policies can validate, mutate, and generate any Kubernetes resources.

Kyverno runs as a [dynamic admission controller](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) in a Kubernetes cluster. Kyverno receives validating and mutating admission webhook HTTP callbacks from the kube-apiserver and applies matching policies to return results that enforce admission policies or reject requests.

Kyverno policies can match resources using the resource kind, name, and label selectors. Wildcards are supported in names.

Mutating policies can be written as overlays (similar to [Kustomize](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/#bases-and-overlays)) or as a [JSON Patch](http://jsonpatch.com/). Validating policies also use an overlay style syntax, with support for pattern matching and conditional (if-then-else) processing.

Policy enforcement is captured using Kubernetes events. Kyverno also reports policy violations for existing resources.

## Examples

### 1. Validating resources

This policy requires that all pods have CPU and memory resource requests and limits:

````yaml
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: check-cpu-memory
spec:
  rules:
  - name: check-pod-resources
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "CPU and memory resource requests and limits are required"
      pattern:
        spec:
          containers:
          # 'name: *' selects all containers in the pod
          - name: "*"
            resources:
              limits:
                # '?' requires 1 alphanumeric character and '*' means that there can be 0 or more characters.
                # Using them together e.g. '?*' requires at least one character. 
                memory: "?*"
                cpu: "?*"
              requests:
                memory: "?*"
                cpu: "?*"
````

### 2. Mutating resources

This policy sets the imagePullPolicy to Always if the image tag is latest:

````yaml
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: set-image-pull-policy
spec:
  rules:
  - name: set-image-pull-policy
    match:
      resources:
        kinds:
        - Deployment
    mutate:
      overlay:
        spec:
          template:
            spec:
              containers:
                # match images which end with :latest   
                - (image): "*:latest"
                  # set the imagePullPolicy to "Always"
                  imagePullPolicy: "Always"
````

### 3. Generating resources

This policy sets the Zookeeper and Kafka connection strings for all namespaces with a label key 'kafka'.

````yaml
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: "zk-kafka-address"
spec:
  rules:
  - name: "zk-kafka-address"
    match:
      resources:
        kinds:
          - Namespace
        selector:
          matchExpressions:
          - {key: kafka, operator: Exists}
    generate:
      kind: ConfigMap
      name: zk-kafka-address
      data:
        kind: ConfigMap
        data:
          ZK_ADDRESS: "192.168.10.10:2181,192.168.10.11:2181,192.168.10.12:2181"
          KAFKA_ADDRESS: "192.168.10.13:9092,192.168.10.14:9092,192.168.10.15:9092"
````

### 4. More examples

Refer to a list of curated of ***[sample policies](/samples/README.md)*** that can be applied to your cluster.

## License

[Apache License 2.0](https://github.com/nirmata/kyverno/blob/master/LICENSE)


## Documentation

* [Getting Started](documentation/installation.md)
* [Writing Policies](documentation/writing-policies.md)
  * [Mutate](documentation/writing-policies-mutate.md)
  * [Validate](documentation/writing-policies-validate.md)
  * [Generate](documentation/writing-policies-generate.md)
* [Testing Policies](documentation/testing-policies.md)
  * [Using kubectl](documentation/testing-policies.md#Test-using-kubectl)
  * [Using the Kyverno CLI](documentation/testing-policies.md#Test-using-the-Kyverno-CLI)


## Alternatives

### Open Policy Agent

[Open Policy Agent (OPA)](https://www.openpolicyagent.org/) is a general-purpose policy engine that can be used as a Kubernetes admission controller. It supports a large set of use cases. Policies are written using [Rego](https://www.openpolicyagent.org/docs/latest/how-do-i-write-policies#what-is-rego) a custom query language.

### k-rail

[k-rail](https://github.com/cruise-automation/k-rail/) provides several ready to use policies for security and multi-tenancy. The policies are written in Golang. Several of the [Kyverno sample policies](/samples/README.md were inspired by k-rail policies.

### Polaris

[Polaris](https://github.com/reactiveops/polaris) validates configurations for best practices. It includes several checks across health, networking, security, etc. Checks can be assigned a severity. A dashboard reports the overall score.

### External configuration management tools

Tools like [Kustomize](https://github.com/kubernetes-sigs/kustomize) can be used to manage variations in configurations outside of clusters. There are several advantages to this approach when used to produce variations of the same base configuration. However, such solutions cannot be used to validate or enforce configurations.


## Roadmap

See [Milestones](https://github.com/nirmata/kyverno/milestones) and [Issues](https://github.com/nirmata/kyverno/issues).

## Getting help

  * For feature requests and bugs, file an [issue](https://github.com/nirmata/kyverno/issues).
  * For discussions or questions, join our [Kubernetes Slack channel #kyverno](https://app.slack.com/client/T09NY5SBT/CLGR9BJU9) or the [mailing list](https://groups.google.com/forum/#!forum/kyverno)

## Contributing

Thanks for your interest in contributing!

  * Please review and agree to abide with the [Code of Conduct](/CODE_OF_CONDUCT.md) before contributing.
  * See the [Wiki](https://github.com/nirmata/kyverno/wiki) for developer documentation.
  * Browse through the [open issues](https://github.com/nirmata/kyverno/issues)
