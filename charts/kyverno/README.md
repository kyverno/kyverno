# kyverno

Kubernetes Native Policy Management

![Version: v2.5.1](https://img.shields.io/badge/Version-v2.5.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.7.1](https://img.shields.io/badge/AppVersion-v1.7.1-informational?style=flat-square)

## About

[Kyverno](https://kyverno.io) is a Kubernetes Native Policy Management engine.

It allows you to:
- Manage policies as Kubernetes resources (no new language required.)
- Validate, mutate, and generate resource configurations.
- Select resources based on labels and wildcards.
- View policy enforcement as events.
- Scan existing resources for violations.

This chart bootstraps a Kyverno deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

Access the complete user documentation and guides at: https://kyverno.io.

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
$ helm install kyverno --namespace kyverno kyverno/kyverno
```

The command deploys Kyverno on the Kubernetes cluster with default configuration. The [installation](https://kyverno.io/docs/installation/) guide lists the parameters that can be configured during installation.

The Kyverno ClusterRole/ClusterRoleBinding that manages webhook configurations must have the suffix `:webhook`. Ex., `*:webhook` or `kyverno:webhook`.
Other ClusterRole/ClusterRoleBinding names are configurable.

## Uninstalling the Chart

To uninstall/delete the `kyverno` deployment:

```console
$ helm delete -n kyverno kyverno
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nameOverride | string | `nil` | Override the name of the chart |
| fullnameOverride | string | `nil` | Override the expanded name of the chart |
| namespace | string | `nil` | Namespace the chart deploys to |
| customLabels | object | `{}` | Additional labels |
| rbac.create | bool | `true` | Create ClusterRoles, ClusterRoleBindings, and ServiceAccount |
| rbac.serviceAccount.create | bool | `true` | Create a ServiceAccount |
| rbac.serviceAccount.name | string | `nil` | The ServiceAccount name |
| rbac.serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| image.repository | string | `"ghcr.io/kyverno/kyverno"` | Image repository |
| image.tag | string | `nil` | Image tag Defaults to appVersion in Chart.yaml if omitted |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.pullSecrets | list | `[]` | Image pull secrets |
| initImage.repository | string | `"ghcr.io/kyverno/kyvernopre"` | Image repository |
| initImage.tag | string | `nil` | Image tag If initImage.tag is missing, defaults to image.tag |
| initImage.pullPolicy | string | `nil` | Image pull policy If initImage.pullPolicy is missing, defaults to image.pullPolicy |
| testImage.repository | string | `nil` | Image repository Defaults to `busybox` if omitted |
| testImage.tag | string | `nil` | Image tag Defaults to `latest` if omitted |
| testImage.pullPolicy | string | `nil` | Image pull policy Defaults to image.pullPolicy if omitted |
| replicaCount | int | `nil` | Desired number of pods |
| podLabels | object | `{}` | Additional labels to add to each pod |
| podAnnotations | object | `{}` | Additional annotations to add to each pod |
| podSecurityContext | object | `{}` | Security context for the pod |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"privileged":false,"readOnlyRootFilesystem":true,"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context for the containers |
| priorityClassName | string | `""` | Optional priority class to be used for kyverno pods |
| antiAffinity.enable | bool | `true` | Pod antiAffinities toggle. Enabled by default but can be disabled if you want to schedule pods to the same node. |
| podAntiAffinity | object | See [values.yaml](values.yaml) | Pod anti affinity constraints. |
| podAffinity | object | `{}` | Pod affinity constraints. |
| nodeAffinity | object | `{}` | Node affinity constraints. |
| podDisruptionBudget.minAvailable | int | `1` | Configures the minimum available pods for kyverno disruptions. Cannot be used if `maxUnavailable` is set. |
| podDisruptionBudget.maxUnavailable | string | `nil` | Configures the maximum unavailable pods for kyverno disruptions. Cannot be used if `minAvailable` is set. |
| nodeSelector | object | `{}` | Node labels for pod assignment |
| tolerations | list | `[]` | List of node taints to tolerate |
| hostNetwork | bool | `false` | Change `hostNetwork` to `true` when you want the kyverno's pod to share its host's network namespace. Useful for situations like when you end up dealing with a custom CNI over Amazon EKS. Update the `dnsPolicy` accordingly as well to suit the host network mode. |
| dnsPolicy | string | `"ClusterFirst"` | `dnsPolicy` determines the manner in which DNS resolution happens in the cluster. In case of `hostNetwork: true`, usually, the `dnsPolicy` is suitable to be `ClusterFirstWithHostNet`. For further reference: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy. |
| envVarsInit | object | `{}` | Env variables for initContainers. |
| envVars | object | `{}` | Env variables for containers. |
| extraArgs | list | `["--autogenInternals=false"]` | Extra arguments to give to the binary. |
| imagePullSecrets | object | `{}` | Image pull secrets for image verify and imageData policies. This will define the `--imagePullSecrets` Kyverno argument. |
| resources.limits | object | `{"memory":"384Mi"}` | Pod resource limits |
| resources.requests | object | `{"cpu":"100m","memory":"128Mi"}` | Pod resource requests |
| initResources.limits | object | `{"cpu":"100m","memory":"256Mi"}` | Pod resource limits |
| initResources.requests | object | `{"cpu":"10m","memory":"64Mi"}` | Pod resource requests |
| livenessProbe | object | See [values.yaml](values.yaml) | Liveness probe. The block is directly forwarded into the deployment, so you can use whatever livenessProbe configuration you want. ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/ |
| readinessProbe | object | See [values.yaml](values.yaml) | Readiness Probe. The block is directly forwarded into the deployment, so you can use whatever readinessProbe configuration you want. ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/ |
| generatecontrollerExtraResources | string | `nil` |  |
| excludeKyvernoNamespace | bool | `true` | Exclude Kyverno namespace Determines if default Kyverno namespace exclusion is enabled for webhooks and resourceFilters |
| config.resourceFilters | list | See [values.yaml](values.yaml) | Resource types to be skipped by the Kyverno policy engine. Make sure to surround each entry in quotes so that it doesn't get parsed as a nested YAML list. These are joined together without spaces, run through `tpl`, and the result is set in the config map. |
| config.existingConfig | string | `""` | Name of an existing config map (ignores default/provided resourceFilters) |
| config.excludeGroupRole | string | `nil` | Exclude group role |
| config.excludeUsername | string | `nil` | Exclude username |
| config.webhooks | string | `nil` | Defines the `namespaceSelector` in the webhook configurations. Note that it takes a list of `namespaceSelector` and/or `objectSelector` in the JSON format, and only the first element will be forwarded to the webhook configurations. The Kyverno namespace is excluded if `excludeKyvernoNamespace` is `true` (default) |
| config.generateSuccessEvents | bool | `false` | Generate success events. |
| config.metricsConfig | object | `{"namespaces":{"exclude":[],"include":[]}}` | Metrics config. |
| updateStrategy | object | See [values.yaml](values.yaml) | Deployment update strategy. Ref: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy |
| service.port | int | `443` | Service port. |
| service.type | string | `"ClusterIP"` | Service type. |
| service.nodePort | string | `nil` | Service node port. Only used if `service.type` is `NodePort`. |
| service.annotations | object | `{}` | Service annotations. |
| topologySpreadConstraints | list | `[]` | Topology spread constraints. |
| metricsService.create | bool | `true` | Create service. |
| metricsService.port | int | `8000` | Service port. Kyverno's metrics server will be exposed at this port. |
| metricsService.type | string | `"ClusterIP"` | Service type. |
| metricsService.nodePort | string | `nil` | Service node port. Only used if `metricsService.type` is `NodePort`. |
| metricsService.annotations | object | `{}` | Service annotations. |
| serviceMonitor.enabled | bool | `false` | Create a `ServiceMonitor` to collect Prometheus metrics. |
| serviceMonitor.additionalLabels | string | `nil` | Additional labels |
| serviceMonitor.namespace | string | `nil` | Override namespace (default is the same as kyverno) |
| serviceMonitor.interval | string | `"30s"` | Interval to scrape metrics |
| serviceMonitor.scrapeTimeout | string | `"25s"` | Timeout if metrics can't be retrieved in given time interval |
| serviceMonitor.secure | bool | `false` | Is TLS required for endpoint |
| serviceMonitor.tlsConfig | object | `{}` | TLS Configuration for endpoint |
| createSelfSignedCert | bool | `false` | Kyverno requires a certificate key pair and corresponding certificate authority to properly register its webhooks. This can be done in one of 3 ways: 1) Use kube-controller-manager to generate a CA-signed certificate (preferred) 2) Provide your own CA and cert.    In this case, you will need to create a certificate with a specific name and data structure.    As long as you follow the naming scheme, it will be automatically picked up.    kyverno-svc.(namespace).svc.kyverno-tls-ca (with data entry named rootCA.crt)    kyverno-svc.kyverno.svc.kyverno-tls-pair (with data entries named tls.key and tls.crt) 3) Let Helm generate a self signed cert, by setting createSelfSignedCert true If letting Kyverno create its own CA or providing your own, make createSelfSignedCert is false |
| installCRDs | bool | `true` | Whether to have Helm install the Kyverno CRDs. If the CRDs are not installed by Helm, they must be added before policies can be created. |
| networkPolicy.enabled | bool | `false` | When true, use a NetworkPolicy to allow ingress to the webhook This is useful on clusters using Calico and/or native k8s network policies in a default-deny setup. |
| networkPolicy.ingressFrom | list | `[]` | A list of valid from selectors according to https://kubernetes.io/docs/concepts/services-networking/network-policies. |
| webhooksCleanup.enable | bool | `false` | Create a helm pre-delete hook to cleanup webhooks. |
| webhooksCleanup.image | string | `"bitnami/kubectl:latest"` | `kubectl` image to run commands for deleting webhooks. |
| tufRootMountPath | string | `"/.sigstore"` | A writable volume to use for the TUF root initialization |

## TLS Configuration

If `createSelfSignedCert` is `true`, Helm will take care of the steps of creating an external self-signed certificate described in option 2 of the [installation documentation](https://kyverno.io/docs/installation/#option-2-use-your-own-ca-signed-certificate)

If `createSelfSignedCert` is `false`, Kyverno will generate a self-signed CA and a certificate, or you can provide your own TLS CA and signed-key pair and create the secret yourself as described in the [documentation](https://kyverno.io/docs/installation/#customize-the-installation-of-kyverno).

## Default resource filters

[Kyverno resource filters](https://kyverno.io/docs/installation/#resource-filters) are a used to exclude resources from the Kyverno engine rules processing.

This chart comes with default resource filters that apply exclusions on a couple of namespaces and resource kinds:
- all resources in `kube-system`, `kube-public` and `kube-node-lease` namespaces
- all resources in all namespaces for the following resource kinds:
  - `Event`
  - `Node`
  - `APIService`
  - `TokenReview`
  - `SubjectAccessReview`
  - `SelfSubjectAccessReview`
  - `Binding`
  - `ReplicaSet`
  - `ReportChangeRequest`
  - `ClusterReportChangeRequest`
- all resources created by this chart itself

Those default exclusions are there to prevent disruptions as much as possible.
Under the hood, Kyverno installs an admission controller for critical cluster resources.
A cluster can become unresponsive if Kyverno is not up and running, ultimately preventing pods to be scheduled in the cluster.

You can however override the default resource filters by setting the `config.resourceFilters` stanza.
It contains an array of string templates that are passed through the `tpl` Helm function and joined together to produce the final `resourceFilters` written in the Kyverno config map.

Please consult the [values.yaml](./values.yaml) file before overriding `config.resourceFilters` and use the apropriate templates to build your desired exclusions list.

## High availability

Running a highly-available Kyverno installation is crucial in a production environment.

In order to run Kyverno in high availability mode, you should set `replicaCount` to `3` or more.
You should also pay attention to anti affinity rules, spreading pods across nodes and availability zones.

Please see https://kyverno.io/docs/installation/#security-vs-operability for more informations.

## Source Code

* <https://github.com/kyverno/kyverno>

## Requirements

Kubernetes: `>=1.16.0-0`

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Nirmata |  | https://kyverno.io/ |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.6.0](https://github.com/norwoodj/helm-docs/releases/v1.6.0)
