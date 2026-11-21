# kyverno-policies

Kubernetes Pod Security Standards implemented as Kyverno policies

![Version: v0.0.0](https://img.shields.io/badge/Version-v0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: latest](https://img.shields.io/badge/AppVersion-latest-informational?style=flat-square)

## About

**1.20 notice.** This chart's built-in PSS templates render `policies.kyverno.io` policy types.
Kyverno 1.20 no longer executes legacy `kyverno.io` policies and rejects their create and update
requests. Anything you pass through `.Values.customPolicies` keeps the group, version, and kind
you supply — the chart renders it verbatim — so a legacy `kyverno.io/v1` `ClusterPolicy` supplied
that way is denied at admission and the release fails to apply. Use a supported
`policies.kyverno.io` kind for custom manifests.

**Review your legacy objects before upgrading.** The `policyType` option is gone, and this chart
converts nothing. If you previously installed it with `policyType: ClusterPolicy`, upgrading
deletes the `ClusterPolicy` objects the release owns, because Helm removes resources that are no
longer in the rendered manifest. The chart installs `ValidatingPolicy` equivalents of its own PSS
policies, so that coverage continues, but the old objects are removed rather than converted and
any legacy manifest you supplied through `.Values.customPolicies` is neither converted nor
replaced. Migrate anything you rely on before you upgrade.

This chart contains Kyverno's implementation of the Kubernetes Pod Security Standards (PSS) as documented at https://kubernetes.io/docs/concepts/security/pod-security-standards/ and are a Helm packaged version of those found at https://github.com/kyverno/policies/tree/main/pod-security. The goal of the PSS controls is to provide a good starting point for general Kubernetes cluster operational security. These controls are broken down into two categories, Baseline and Restricted. Baseline policies implement the most basic of Pod security controls while Restricted implements more strict controls. Restricted is cumulative and encompasses those listed in Baseline.

The following policies are included in each profile.

**Baseline**

* disallow-capabilities
* disallow-host-namespaces
* disallow-host-path
* disallow-host-ports
* disallow-host-probes
* disallow-host-process
* disallow-privileged-containers
* disallow-proc-mount
* disallow-selinux
* restrict-apparmor-profiles
* restrict-seccomp
* restrict-sysctls

**Restricted**

* disallow-capabilities-strict
* disallow-privilege-escalation
* disallow-proc-mount-strict
* require-run-as-non-root-user
* require-run-as-nonroot
* restrict-seccomp-strict
* restrict-volume-types

An additional policy "require-non-root-groups" is included in an `other` group as this was previously included in the official PSS controls but since removed.

For the latest version of these PSS policies, always refer to the kyverno/policies repo at https://github.com/kyverno/policies/tree/main/pod-security.

## Select a Pod Security Admission version

Kubernetes Pod Security Admission (PSA) versions its checks, and a namespace can pin a version with the `pod-security.kubernetes.io/enforce-version` label. The `podSecurityVersion` value does the same for the Pod Security Standards policies in this chart.

* `latest` (default) matches the cluster's Kubernetes minor version when you install or upgrade the chart. It keeps the allow-lists of earlier chart releases, so Pods that pass today keep passing.
* A version from `v1.25` to `v1.37`, for example `v1.34`, matches that PSA version. A version newer than `v1.37` uses `v1.37`, and an older or malformed version fails the install.

```console
$ helm install kyverno-policies --namespace kyverno kyverno/kyverno-policies \
    --set podSecurityVersion=v1.34
```

Each policy that changes between PSA versions shows the version it matches in its `kyverno.io/pod-security-version` annotation:

* A pinned version shows as the version, for example `v1.34`. The policies match PSA for that version.
* `latest` shows as `latest` and the resolved version, for example `latest (v1.30)`. The version-gated checks, such as `disallow-host-probes` and the `v1.32` and `v1.37` sysctls, follow that version. The legacy allow-lists still apply, so `latest` can allow more than PSA `v1.30` does. For example, it allows the `container_engine_t` SELinux type, which PSA allows only from `v1.31`. To match PSA exactly, pin a version.

Because Helm resolves `latest` only when it renders the chart, run `helm upgrade` after you upgrade Kubernetes to pick up newer checks.

These policies change between PSA versions. The other policies are the same for every supported version.

| Policy | PSA versions with a change | Change |
|---|---|---|
| `restrict-sysctls` | `v1.27`, `v1.29`, `v1.32`, `v1.37` | Each version allows more safe sysctls. With `latest`, the chart allows at least the `v1.29` list. |
| `disallow-selinux` | `v1.31` | Allows the `container_engine_t` type. Always allowed with `latest`. |
| `disallow-host-probes` | `v1.34` | Installed from `v1.34`. |
| `disallow-proc-mount`, `require-run-as-nonroot`, `require-run-as-non-root-user` | `v1.35` | Skip Pods that set `spec.hostUsers` to `false`, when `podSecurityUserNamespaces` is `true`. With `latest`, `podSecurityUserNamespaces` alone decides. |
| `disallow-proc-mount-strict` | `v1.35` | Installed from `v1.35` with the Restricted profile, when `podSecurityUserNamespaces` is `true`. |

## Deploy custom policies
If you have custom policies you would like to deploy as part of the Helm release, provide their manifests in `.Values.customPolicies`:
````yaml
customPolicies:
  - apiVersion: policies.kyverno.io/v1beta1
    kind: ValidatingPolicy
    metadata: # metadata
    spec: # spec
````

## Installing the Chart

These PSS policies presently have a minimum requirement of Kyverno 1.17.0.

```console
## Add the Kyverno Helm repository
$ helm repo add kyverno https://kyverno.github.io/kyverno/

## Install the Kyverno Policies Helm chart
$ helm install kyverno-policies --namespace kyverno kyverno/kyverno-policies
```

## Uninstalling the Chart

To uninstall/delete the `kyverno-policies` chart:

```console
$ helm delete -n kyverno kyverno-policies
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| podSecurityStandard | string | `"baseline"` | Pod Security Standard profile (`baseline`, `restricted`, `privileged`, `custom`). For more info https://kyverno.io/policies/pod-security. |
| podSecuritySeverity | string | `"medium"` | Pod Security Standard severity (`low`, `medium`, `high`). |
| podSecuritySeverityByPolicy | object | `{}` | Define podSecuritySeverity overrides for specific policies. Override the global `podSecuritySeverity` with an individual severity for individual policies. An empty string per-policy entry suppresses the annotation entirely. |
| podSecurityPolicies | list | `[]` | Policies to include when `podSecurityStandard` is `custom`. |
| podSecurityUserNamespaces | bool | `false` | Enable the Kubernetes Pod Security Admission relaxation for user namespace pods. When `true`, the `disallow-proc-mount` (Baseline), `require-run-as-nonroot` and `require-run-as-non-root-user` (Restricted) ValidatingPolicies skip Pods that explicitly set `spec.hostUsers` to `false`. A Pod that omits `hostUsers` or sets it to `true` is always checked. The Restricted `disallow-proc-mount-strict` ValidatingPolicy never relaxes, and neither does `disallow-proc-mount` when the Restricted profile is installed. When `podSecurityVersion` is pinned below `v1.35`, the relaxation does not apply, because Kubernetes Pod Security Admission added it in `v1.35`. From `v1.35`, Pod Security Admission always applies the relaxation, so with this set to `false` a pinned `v1.35` or later denies some `hostUsers: false` Pods that Pod Security Admission allows. |
| podSecurityVersion | string | `"latest"` | Kubernetes Pod Security Admission version that the Pod Security Standards policies match, the same as the `pod-security.kubernetes.io/enforce-version` namespace label. Use `latest` or a version from `v1.25` to `v1.37`, for example `v1.34`. A version newer than `v1.37` uses `v1.37`. `latest` uses the cluster's Kubernetes minor version when you install or upgrade the chart, and keeps the allow-lists of earlier chart releases, so Pods that pass today keep passing. Because of that, `latest` can allow more than Pod Security Admission does for that version. Each version-aware policy shows the version in its `kyverno.io/pod-security-version` annotation, for example `v1.34` for a pinned version or `latest (v1.30)` for `latest` on Kubernetes 1.30. |
| includeOtherPolicies | list | `[]` | Additional policies to include from `other`. |
| includeRestrictedPolicies | list | `[]` | Additional policies to include from `restricted`. |
| customPolicies | list | `[]` | Additional custom policies to include. |
| failurePolicy | string | `"Fail"` | API server behavior if the webhook fails to respond ('Ignore', 'Fail') For more info: https://kyverno.io/docs/policy-types/cluster-policy/policy-settings/ |
| validationFailureAction | string | `"Audit"` | Validation failure action (`Audit`, `Enforce`). For more info https://kyverno.io/docs/policy-types/cluster-policy/validate. |
| validationFailureActionByPolicy | object | `{}` | Define validationFailureActionByPolicy for specific policies. Override the defined `validationFailureAction` with a individual validationFailureAction for individual Policies. |
| auditAnnotations | object | `{}` | Default audit annotations applied to all ValidatingPolicy policies. Map of annotation key to CEL valueExpression. Audit annotations are recorded in the API server audit log. For more info https://kyverno.io/docs/policy-types/validating-policy/#using-auditannotations-to-add-custom-data |
| auditAnnotationsByPolicy | object | `{}` | Define audit annotations for specific ValidatingPolicy policies. Per-policy entries override defaults when they share the same key. |
| vpolExclude | object | `{}` | Default excludes applied to ALL ValidatingPolicy policies. Supports the following optional keys:   excludeResourceRules: list of Kubernetes NamedRuleWithOperations (native VAP excludes)   excludeNamespaces:    list of namespace names to exclude (generates CEL matchCondition)   excludeSubjects:      list of subjects to exclude (generates CEL matchCondition)   matchConditions:      list of CEL matchConditions for advanced filtering (passthrough of custom condition) Per-policy overrides via vpolExcludeByPolicy replace these defaults entirely for that policy. |
| vpolExcludeByPolicy | object | `{}` | Per-policy excludes for individual ValidatingPolicy policies. When set for a policy, it completely replaces the global vpolExclude defaults for that policy. Each policy name maps to an object with the same keys as vpolExclude. |
| autogenControllers | string | `""` | Customize the target Pod controllers for the auto-generated rules. (Eg. `none`, `Deployment`, `DaemonSet,Deployment,StatefulSet`) This sets `spec.autogen.podControllers.controllers`, and only `DaemonSet`, `Deployment`, `ReplicaSet`, `StatefulSet`, `Job` and `CronJob` are supported. Any other name fails the render, except `ReplicationController`, which is accepted and ignored. A list without any supported controller fails the render, use `none` to turn autogen off. For more info https://kyverno.io/docs/policy-types/cluster-policy/autogen/. |
| nameOverride | string | `nil` | Name override. |
| customAnnotations | object | `{}` | Additional Annotations. |
| customAnnotationsByPolicy | object | `{}` | Define custom annotations for specific policies. Per-policy entries override defaults when they share the same key. |
| customLabels | object | `{}` | Additional labels. |
| kyvernoVersion | string | `"autodetect"` | Kyverno version The default of "autodetect" will try to determine the currently installed version from the deployment |
| kubeVersionOverride | string | `nil` | Kubernetes version override Override default value of kubeVersion set by release team taken from Chart.yaml with custom value. Ideally range of versions no more than two prior (ex., 1.28-1.31), must be enclosed in quotes. |

## Source Code

* <https://github.com/kyverno/policies>

## Requirements

Kubernetes: `>=1.25.0-0`

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| kyverno-maintainers | <cncf-kyverno-maintainers@lists.cncf.io> |  |

## Changes

### v2.3.4

* Do not evaluate `foreach` policies on DELETE

### v2.3.3

* Add policyPreconditions value to allow policies and rules to have preconditions added

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
