## Description

This test verifies that a namespaced `Policy` cannot use `{{...}}` variables or `$(...)` references in generate fields that select the target type or the target or source namespace. It covers direct generation, clone, cloneList, and foreach. A reference can copy a permitted dynamic name into a scope field before variable substitution, so both expression forms must be rejected there.

Generated resource names remain dynamic because they cannot change the selected type or namespace. The passing cases use a variable directly in the name and a name reference to a variable in generated data.

These namespaced scope restrictions do not apply to `ClusterPolicy` structural validation. Live admission still requires static target kinds for authorization, including `ClusterPolicy`; offline CLI validation retains its existing support for cluster-scoped policy templates.

The CEL-based `GeneratingPolicy` and `NamespacedGeneratingPolicy` APIs are not covered by this validation. They use `policies.kyverno.io` generation expressions or templates and do not use the legacy `kyverno.io/v1` `GeneratePattern` fields exercised here.

## Expected behavior

Policies with dynamic generated resource names are accepted. Policies with variable or reference target kinds, API versions, target namespaces, clone source namespaces, cloneList kinds or namespaces, and foreach target kinds are rejected during admission.

The regression in `pkg/policy/generate/scope_reference_test.go` also exercises the offline CLI validation path without discovery or RBAC. It proves that both relative and absolute references can resolve a dynamic name into a target kind, and that the original namespaced policy is rejected before that substitution.
