# Kyverno Policies

## TL;DR

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
| `podSecurityStandard`              | set desired pod security level `privileged`, `baseline`, `restricted`, `custom`. Set to `restricted` for maximum security for your cluster. See: https://kyverno.io/policies/pod-security/                                                               | `baseline`                                                                                                                                                                                                                                                               |
| `podSecuritySeverity`              | set desired pod security severity `low`, `medium`, `high`. Used severity level in PolicyReportResults for the selected pod security policies.                                                                                                            | `medium`                                                                                                                                                                                                                                                                 |
| `podSecurityPolicies`              | Policies to include when `podSecurityStandard` is set to `custom`                                                                                                                                                                                        | `[]`                                                                                                                                                                                                                                                                     |
| `validationFailureAction`          | set to get response in failed validation check. Supported values are `audit` and `enforce`. See: https://kyverno.io/docs/writing-policies/validate/                                                                                                      | `audit`                                                                                                                                                                                                                                                                  |

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
