# Reporting for generated native admission policies

Native ValidatingAdmissionPolicy (VAP) and MutatingAdmissionPolicy (MAP)
reporting is opt-in for each policy. The reports controller must also have
`--validatingAdmissionPolicyReports=true` or
`--mutatingAdmissionPolicyReports=true`, respectively. Enabling the controller
flag alone does not select a policy for reporting.

For a Kyverno-generated policy, set `reports.kyverno.io/enabled: "true"` on
its source ValidatingPolicy, MutatingPolicy, or supported CEL ClusterPolicy.
The generator propagates the reporting choice to the native policy. For example:

```yaml
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-env
  labels:
    reports.kyverno.io/enabled: "true"
spec:
  autogen:
    validatingAdmissionPolicy:
      enabled: true
  validationActions: [Audit]
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE, UPDATE]
      resources: [configmaps]
  validations:
  - expression: "has(object.metadata.labels) && 'env' in object.metadata.labels"
    message: ConfigMaps must have an env label
```

For MutatingPolicy, use the same metadata label with
`spec.autogen.mutatingAdmissionPolicy.enabled: true`. See the
[generated MAP conformance example](../../../test/conformance/chainsaw/mutating-admission-policy-reports/background/generated-reporting-opt-in/policy.yaml)
for a complete policy; add the enabled label to opt in.

Removing the enabled label, or setting it to a value other than `"true"`,
removes generated reporting enablement. A `reports.kyverno.io/disabled` label
with **any** value overrides enablement. Removing that disabled label restores
reporting only if the source still has enabled set to `"true"`. Label-only
source updates trigger reconciliation; changing the policy spec is unnecessary.
Other labels on the generated policy are preserved.

The source is authoritative for reporting labels on generated policies. Put
persistent reporting configuration on the source, rather than editing the
native child. For independently authored native VAPs/MAPs, apply the enabled
label directly to the native policy. The managed-by label alone never opts in.

Reports describe Kyverno's background evaluation of existing resources against
the native policy and its applicable bindings. Namespaced resources produce
PolicyReport results; cluster-scoped resources produce ClusterPolicyReport
results. Reconciliation is asynchronous. Results are not a replay of the API
server's admission warnings or audit events, and a warning does not guarantee
an immediate report. Background scanning also requires reports-controller
permissions for the matched resources.
