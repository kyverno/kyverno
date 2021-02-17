# Kyverno

[Kyverno](https://kyverno.io) is a Kubernetes Native Policy Management engine. It allows you to:

* Manage policies as Kubernetes resources (no new language required.)
* Validate, mutate, and generate resource configurations.
* Select resources based on labels and wildcards.
* View policy enforcement as events.
* Scan existing resources for violations.

Access the complete user documentation and guides at: https://kyverno.io.

## TL;DR

```console
## Add the Kyverno Helm repository
$ helm repo add kyverno https://kyverno.github.io/kyverno/

## Install the Kyverno Helm chart
$ helm install kyverno --namespace kyverno kyverno/kyverno --create-namespace
```

## Introduction

This chart bootstraps a Kyverno deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Installing the Chart

**Add the Kyverno Helm repository:**

```console
$ helm repo add kyverno https://kyverno.github.io/kyverno/
```

**Create a namespace:**

You can install Kyverno in any namespace. The examples use `kyverno` as the namespace.

```console
$ kubectl create namespace kyverno
```

**Install the Kyverno chart:**

```console
$ helm install kyverno --namespace kyverno kyverno ./charts/kyverno
```

The command deploys Kyverno on the Kubernetes cluster with default configuration. The [installation](https://kyverno.io/docs/installation/) guide lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `kyverno` deployment:

```console
$ helm delete -n kyverno kyverno
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the kyverno chart and their default values.

Parameter | Description | Default
--- | --- | ---
`affinity` | node/pod affinities | `nil`
`createSelfSignedCert` | generate a self signed cert and certificate authority. Kyverno defaults to using kube-controller-manager CA-signed certificate or existing cert secret if false. | `false`
`config.existingConfig` | existing Kubernetes configmap to use for the resource filters configuration | `nil`
`config.resourceFilters` | list of filter of resource types to be skipped by kyverno policy engine. See [documentation](https://github.com/kyverno/kyverno/blob/master/documentation/installation.md#filter-kubernetes-resources-that-admission-webhook-should-not-process) for details | `["[Event,*,*]","[*,kube-system,*]","[*,kube-public,*]","[*,kube-node-lease,*]","[Node,*,*]","[APIService,*,*]","[TokenReview,*,*]","[SubjectAccessReview,*,*]","[*,kyverno,*]"]`
`dnsPolicy` | Sets the DNS Policy which determines the manner in which DNS resolution happens across the cluster. For further reference, see [the official docs](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy) | `ClusterFirst`
`extraArgs` | list of extra arguments to give the binary | `[]`
`fullnameOverride` | override the expanded name of the chart | `nil`
`generatecontrollerExtraResources` | extra resource type Kyverno is allowed to generate | `[]`
`hostNetwork` | Use the host network's namespace. Set it to `true` when dealing with a custom CNI over Amazon EKS | `false`
`image.pullPolicy` | Image pull policy | `IfNotPresent`
`image.pullSecrets` | Specify image pull secrets | `[]` (does not add image pull secrets to deployed pods)
`image.repository` | Image repository | `ghcr.io/kyverno/kyverno`
`image.tag` | Image tag | `nil`
`initImage.pullPolicy` | Init image pull policy | `nil`
`initImage.repository` | Init image repository | `ghcr.io/kyverno/kyvernopre`
`initImage.tag` | Init image tag | `nil`
`livenessProbe` | liveness probe configuration | `{}`
`nameOverride` | override the name of the chart | `nil`
`namespace` | namespace the chart deploy to | `nil`
`nodeSelector` | node labels for pod assignment | `{}`
`podAnnotations` | annotations to add to each pod | `{}`
`podLabels` | additional labels to add to each pod | `{}`
`podSecurityContext` | security context for the pod | `{}`
`priorityClassName` | priorityClassName | `nil`
`rbac.create` | create cluster roles, cluster role bindings, and service account | `true`
`rbac.serviceAccount.create` | create a service account | `true`
`rbac.serviceAccount.name` | the service account name | `nil`
`rbac.serviceAccount.annotations` | annotations for the service account | `{}`
`readinessProbe` | readiness probe configuration | `{}`
`replicaCount` | desired number of pods | `1`
`resources` | pod resource requests & limits | `{}`
`service.annotations` | annotations to add to the service | `{}`
`service.nodePort` | node port | `nil`
`service.port` | port for the service | `443`
`service.type` | type of service | `ClusterIP`
`tolerations` | list of node taints to tolerate | `[]`
`securityContext` | security context configuration | `{}`
`podSecurityStandard` | set desired pod security level `privileged`, `default`, `restricted`. Set to `restricted` for maximum security for your cluster. See:  https://kyverno.io/policies/pod-security/ | `default`
`validationFailureAction` | set to get response in failed validation check. Supported values- `audit`, `enforce`. See:  https://kyverno.io/docs/writing-policies/validate/ | `audit`

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install --namespace kyverno kyverno ./charts/kyverno \
  --set=image.tag=v0.0.2,resources.limits.cpu=200m
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install --namespace kyverno kyverno ./charts/kyverno -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)

## TLS Configuration

If `createSelfSignedCert` is `true`, Helm will take care of the steps of creating an external self-signed certificate describe in option 2 of the [installation documentation](https://github.com/kyverno/kyverno/blob/master/documentation/installation.md#option-2-use-your-own-ca-signed-certificate)

If `createSelfSignedCert` is `false`, Kyverno will generate a self-signed CA and a certificate, or you can provide your own TLS CA and signed-key pair and create the secret yourself as described in the documentation.

## Kyverno CLI

See: https://kyverno.io/docs/kyverno-cli/