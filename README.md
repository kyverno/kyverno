# Kyverno - Kubernetes Native Policy Management

[![Build Status](https://travis-ci.org/nirmata/kyverno.svg?branch=master)](https://travis-ci.org/nirmata/kyverno) [![Go Report Card](https://goreportcard.com/badge/github.com/nirmata/kyverno)](https://goreportcard.com/report/github.com/nirmata/kyverno) ![License: Apache-2.0](https://img.shields.io/github/license/nirmata/kyverno?color=blue)

![logo](documentation/images/Kyverno_Horizontal.png)

Kyverno is a policy engine built for Kubernetes:
* policies as Kubernetes resources (no new language to learn!)
* validate, mutate, or generate any resource
* match resources using label selectors and wildcards
* validate and mutate using overlays (like Kustomize!)
* generate and synchronize defaults across namespaces
* block or report violations 
* test using kubectl 

Watch a 3 minute video review of Kyverno on Coffee and Cloud Native with [Adrian Goins](https://github.com/oskapt):

[![Kyyverno review on Coffee and Cloud Native](https://img.youtube.com/vi/DW2u6LhNMh0/0.jpg)](https://www.youtube.com/watch?v=DW2u6LhNMh0&feature=youtu.be&t=116)


## Quick Start

**NOTE** : Your Kubernetes cluster version must be above v1.14 which adds webhook timeouts. 
To check the version, enter `kubectl version`.

Install Kyverno:
```console
kubectl create -f https://raw.githubusercontent.com/nirmata/kyverno/master/definitions/release/install.yaml
```

You can also install Kyverno using a [Helm chart](https://github.com/nirmata/kyverno/blob/master/documentation/installation.md#install-kyverno-using-helm).

Add the policy below. It contains a single validation rule that requires that all pods have 
a `app.kubernetes.io/name` label. Kyverno supports different rule types to validate, 
mutate, and generate configurations. The policy attribute `validationFailureAction` is set 
to `enforce` to block API requests that are non-compliant (using the default value `audit` 
will report violations but not block requests.)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: enforce
  rules:
  - name: check-for-labels
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "label `app.kubernetes.io/name` is required"
      pattern:
        metadata:
          labels:
            app.kubernetes.io/name: "?*"
```

Try creating a deployment without the required label:

```console
kubectl create deployment nginx --image=nginx
```

You should see an error:
```console
Error from server: admission webhook "nirmata.kyverno.resource.validating-webhook" denied the request:

resource Deployment/default/nginx was blocked due to the following policies

require-labels:
  autogen-check-for-labels: 'Validation error: label `app.kubernetes.io/name` is required;
    Validation rule autogen-check-for-labels failed at path /spec/template/metadata/labels/app.kubernetes.io/name/'
```

Create a pod with the required label. For example from this YAML:
```yaml
kind: "Pod"
apiVersion: "v1"
metadata:
  name: nginx
  labels:
    app.kubernetes.io/name: nginx
spec:
  containers:
  - name: "nginx"
    image: "nginx:latest"
```

This pod configuration complies with the policy rules, and is not blocked. 

Clean up by deleting all cluster policies:

```console
kubectl delete cpol --all
```

As a next step, browse the [sample policies](https://github.com/nirmata/kyverno/blob/master/samples/README.md) 
and learn about [writing policies](https://github.com/nirmata/kyverno/blob/master/documentation/writing-policies.md). 
You can test policies using the [Kyverno cli](https://github.com/nirmata/kyverno/blob/master/documentation/kyverno-cli.md). 
See [docs](https://github.com/nirmata/kyverno/#documentation) for complete details.

## Documentation

- [Getting Started](documentation/installation.md)
- [Writing Policies](documentation/writing-policies.md)
  - [Selecting Resources](/documentation/writing-policies-match-exclude.md)
  - [Validating Resources](documentation/writing-policies-validate.md)
  - [Mutating Resources](documentation/writing-policies-mutate.md)
  - [Generating Resources](documentation/writing-policies-generate.md)
  - [Variable Substitution](documentation/writing-policies-variables.md)
  - [Preconditions](documentation/writing-policies-preconditions.md)
  - [Auto-Generation of Pod Controller Policies](documentation/writing-policies-autogen.md)
  - [Background Processing](documentation/writing-policies-background.md)
  - [Configmap Lookup](documentation/writing-policies-configmap-reference.md)
- [Testing Policies](documentation/testing-policies.md)
- [Policy Violations](documentation/policy-violations.md)
- [Kyverno CLI](documentation/kyverno-cli.md)
- [Sample Policies](/samples/README.md)
- [API Documentation](https://htmlpreview.github.io/?https://github.com/nirmata/kyverno/blob/master/documentation/index.html)

## License

[Apache License 2.0](https://github.com/nirmata/kyverno/blob/master/LICENSE)

## Community

### Community Meetings

To attend our next monthly community meeting join the [Kyverno group](https://groups.google.com/g/kyverno). You will then be sent a meeting invite and get access to the [agenda and meeting notes](https://docs.google.com/document/d/10Hu1qTip1KShi8Lf_v9C5UVQtp7vz_WL3WVxltTvdAc/edit#).

### Getting Help

- For feature requests and bugs, file an [issue](https://github.com/nirmata/kyverno/issues).
- For discussions or questions, join the **#kyverno** channel on the [Kubernetes Slack](https://kubernetes.slack.com/) or the [mailing list](https://groups.google.com/g/kyverno).

### Contributing

Thanks for your interest in contributing!

- Please review and agree to abide with the [Code of Conduct](/CODE_OF_CONDUCT.md) before contributing.
- We encourage all contributions and encourage you to read our [contribution guidelines](./CONTRIBUTING.md).
- See the [Wiki](https://github.com/nirmata/kyverno/wiki) for developer documentation.
- Browse through the [open issues](https://github.com/nirmata/kyverno/issues)

## Presentations and Articles

- [Introducing Kyverno - blog post](https://nirmata.com/2019/07/11/managing-kubernetes-configuration-with-policies/)
- [CNCF Video and Slides](https://www.cncf.io/webinars/how-to-keep-your-clusters-safe-and-healthy/)
- [10 Kubernetes Best Practices - blog post](https://thenewstack.io/10-kubernetes-best-practices-you-can-easily-apply-to-your-clusters/)
- [VMware Code Meetup Video](https://www.youtube.com/watch?v=mgEmTvLytb0)
- [Virtual Rejekts Video](https://www.youtube.com/watch?v=caFMtSg4A6I)
- [TGIK Video](https://www.youtube.com/watch?v=ZE4Zu9WQET4&list=PL7bmigfV0EqQzxcNpmcdTJ9eFRPBe-iZa&index=18&t=0s)


## Alternatives

### Open Policy Agent

[Open Policy Agent (OPA)](https://www.openpolicyagent.org/) is a general-purpose policy engine that can be used as a Kubernetes admission controller. It supports a large set of use cases. Policies are written using [Rego](https://www.openpolicyagent.org/docs/latest/how-do-i-write-policies#what-is-rego) a custom query language.

### k-rail

[k-rail](https://github.com/cruise-automation/k-rail/) provides several ready to use policies for security and multi-tenancy. The policies are written in Golang. Several of the [Kyverno sample policies](/samples/README.md) were inspired by k-rail policies.

### Polaris

[Polaris](https://github.com/reactiveops/polaris) validates configurations for best practices. It includes several checks across health, networking, security, etc. Checks can be assigned a severity. A dashboard reports the overall score.

### External configuration management tools

Tools like [Kustomize](https://github.com/kubernetes-sigs/kustomize) can be used to manage variations in configurations outside of clusters. There are several advantages to this approach when used to produce variations of the same base configuration. However, such solutions cannot be used to validate or enforce configurations.

## Roadmap

See [Milestones](https://github.com/nirmata/kyverno/milestones) and [Issues](https://github.com/nirmata/kyverno/issues).

