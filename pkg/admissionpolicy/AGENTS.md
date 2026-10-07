# AGENTS.md — pkg/admissionpolicy

Builds native Kubernetes admission policies (ValidatingAdmissionPolicy, MutatingAdmissionPolicy and their bindings)
from Kyverno policies. The `admissionpolicy-generator` controller in `pkg/controllers/admissionpolicygenerator` calls
these builders and owns create/update/delete of the generated objects.

## Generated objects are cluster-scoped — keep these invariants for namespaced policies

A `NamespacedValidatingPolicy` only applies to resources in its own namespace on the webhook path
(`pkg/webhooks/resource/vpol` filters with `NamespacedPolicy(request.Namespace)`). The generated VAP is
cluster-scoped, so the builder has to reproduce that scoping explicitly:

- **Pin the namespace.** `matchConstraints.namespaceSelector` gets a `kubernetes.io/metadata.name In [<namespace>]`
  requirement added to a *copy* of the policy's own selector. Keep the policy's selector: the engine matcher
  (`pkg/cel/matching/matcher.go`) still applies it, so the webhook path enforces "namespace AND selector".
- **Namespaced rules only.** A namespace selector does not restrict cluster-scoped resources, so
  `namespacedResourceRules` drops rules scoped to `Cluster` and sets the rest to `Namespaced`. A policy left without
  rules (or without match constraints) cannot be generated — check `CanGenerateFromValidatingPolicy` before building
  so the controller can delete a previously generated pair instead of failing.
- **No owner reference.** A cluster-scoped object cannot have a namespaced owner; the garbage collector treats it as
  missing. `setSourcePolicy` records the source in the `policies.kyverno.io/source-policy-namespace` and
  `policies.kyverno.io/source-policy-name` annotations instead, and the controller deletes the generated pair itself
  when the policy is gone. Cluster-scoped policies keep a normal owner reference.
- **Never mutate the source policy.** Deep copy match constraints, rules and selectors before changing them — the
  policy comes from an informer cache.

## Generated names

Always use `ValidatingPolicyVAPName(namespace, name)` for both the VAP and (with the `-binding` suffix) the binding:

- `vpol-<name>` for a `ValidatingPolicy`, `nvpol-<namespace>.<name>` for a `NamespacedValidatingPolicy`. Namespace
  names cannot contain dots, so the `.` separator keeps names from different namespace/name pairs distinct.
- Names are validated by the API server as DNS-1123 subdomains (253 characters). A name that would not leave room
  for `-binding` is shortened to `vpolh-`/`nvpolh-`, a readable prefix and a hash of the source, so it stays valid
  and deterministic. Only shortened names use the `h-` prefixes, so they cannot collide with an unshortened name.

Changing the scheme renames generated objects in existing clusters, so the old names would need cleaning up.
