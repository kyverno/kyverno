# kyverno-policies

Kubernetes Pod Security Standards implemented as Kyverno policies

![Version: 2.7.1-rc.1](https://img.shields.io/badge/Version-2.7.1--rc.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.9.1-rc.1](https://img.shields.io/badge/AppVersion-v1.9.1--rc.1-informational?style=flat-square)

## About

This chart contains Kyverno's implementation of the Kubernetes Pod Security Standards (PSS) as documented at https://kubernetes.io/docs/concepts/security/pod-security-standards/ and are a Helm packaged version of those found at https://github.com/kyverno/policies/tree/main/pod-security. The goal of the PSS controls is to provide a good starting point for general Kubernetes cluster operational security. These controls are broken down into two categories, Baseline and Restricted. Baseline policies implement the most basic of Pod security controls while Restricted implements more strict controls. Restricted is cumulative and encompasses those listed in Baseline.

The following policies are included in each profile.

**Baseline**

* disallow-capabilities
* disallow-host-namespaces
* disallow-host-path
* disallow-host-ports
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
* require-run-as-non-root-user
* require-run-as-nonroot
* restrict-seccomp-strict
* restrict-volume-types

An additional policy "require-non-root-groups" is included in an `other` group as this was previously included in the official PSS controls but since removed.

For the latest version of these PSS policies, always refer to the kyverno/policies repo at https://github.com/kyverno/policies/tree/main/pod-security.

## Installing the Chart

These PSS policies presently have a minimum requirement of Kyverno 1.6.0.

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
| podSecuritySeverity | string | `"medium"` | Pod Security Standard (`low`, `medium`, `high`). |
| podSecurityPolicies | list | `[]` | Policies to include when `podSecurityStandard` is `custom`. |
| includeOtherPolicies | list | `[]` | Additional policies to include from `other`. |
| includeRestrictedPolicies | list | `[]` | Additional policies to include from `restricted`. |
| failurePolicy | string | `"Fail"` | API server behavior if the webhook fails to respond ('Ignore', 'Fail') For more info: https://kyverno.io/docs/writing-policies/policy-settings/ |
| validationFailureAction | string | `"audit"` | Validation failure action (`audit`, `enforce`). For more info https://kyverno.io/docs/writing-policies/validate. |
| validationFailureActionByPolicy | object | `{}` | Define validationFailureActionByPolicy for specific policies. Override the defined `validationFailureAction` with a individual validationFailureAction for individual Policies. |
| validationFailureActionOverrides | object | `{"all":[]}` | Define validationFailureActionOverrides for specific policies. The overrides for `all` will apply to all policies. |
| policyExclude | object | `{}` | Exclude resources from individual policies. Policies with multiple rules can have individual rules excluded by using the name of the rule as the key in the `policyExclude` map. |
| policyPreconditions | object | `{}` | Add preconditions to individual policies. Policies with multiple rules can have individual rules excluded by using the name of the rule as the key in the `policyPreconditions` map. |
| autogenControllers | string | `""` | Customize the target Pod controllers for the auto-generated rules. (Eg. `none`, `Deployment`, `DaemonSet,Deployment,StatefulSet`) For more info https://kyverno.io/docs/writing-policies/autogen/. |
| nameOverride | string | `nil` | Name override. |
| customLabels | object | `{}` | Additional labels. |
| background | bool | `true` | Policies background mode |
| kyvernoVersion | string | `"autodetect"` | Kyverno version The default of "autodetect" will try to determine the currently installed version from the deployment |

## Source Code

* <https://github.com/kyverno/policies>

## Requirements

Kubernetes: `>=1.16.0-0`

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Nirmata |  | <https://kyverno.io/> |

## Changes

### v2.3.4

* Do not evaluate `foreach` policies on DELETE

### v2.3.3

* Add policyPreconditions value to allow policies and rules to have preconditions added

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.0](https://github.com/norwoodj/helm-docs/releases/v1.11.0)
