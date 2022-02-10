# Kyverno Policies

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

## TL;DR Instructions

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

## Configuration

The following table lists the configurable parameters of the kyverno chart and their default values.

| Parameter                          | Description                                                                                                                                                                                                                                              | Default                                                                                                                                                                                                                                                                  |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `includeOtherPolicies`              | Additional policies to include from `other` directory                                                              | `[]`                                                                                                                                                                                                                                                               |
| `podSecurityStandard`              | set desired pod security level `privileged`, `baseline`, `restricted`, `custom`. Set to `restricted` for maximum security for your cluster. See: https://kyverno.io/policies/pod-security/                                                               | `baseline`                                                                                                                                                                                                                                                               |
| `podSecuritySeverity`              | set desired pod security severity `low`, `medium`, `high`. Used severity level in PolicyReportResults for the selected pod security policies.                                                                                                            | `medium`                                                                                                                                                                                                                                                                 |
| `podSecurityPolicies`              | Policies to include when `podSecurityStandard` is set to `custom`                                                                                                                                                                                        | `[]`                                                                                                                                                                                                                                                                     |
| `policyExclude`              | Exclude resources from individual policies                                                                                                                                                                                        | `{}`                                                                                                                                                                                                                                                                     |
| `validationFailureAction`          | set to get response in failed validation check. Supported values are `audit` and `enforce`. See: https://kyverno.io/docs/writing-policies/validate/                                                                                                      | `audit`                                                                                                                                                                                                                                                                  |
| `validationFailureActionOverrides`          | Set validate failure action overrides to either all policies or select policies. See: https://kyverno.io/docs/writing-policies/validate/                                                                                                      | `{}`                                                                                                                                                                                                                                                                  |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install --namespace kyverno kyverno-policies ./charts/kyverno-policies \
  --set=podSecurityStandard=restricted,validationFailureAction=enforce
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install --namespace kyverno kyverno-policies ./charts/kyverno-policies -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)
