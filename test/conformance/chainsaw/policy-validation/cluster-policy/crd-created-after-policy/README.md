## Description

This test verifies that a ClusterPolicy using wildcard resource matching (`*/*/*`) correctly enforces validation rules on custom resources whose CRD is created **after** the policy. The policy denies deletion of resources with the label `kyverno.io/critical: "true"`.

## Expected Behavior

1. ClusterPolicy `deny-delete-test-resources` is created and becomes ready
2. A new CustomResourceDefinition (`TestResource`) is registered after the policy exists
3. A custom resource with label `kyverno.io/critical: "true"` is created
4. Attempting to delete the custom resource is denied by the policy

## Related Issue

[kyverno/kyverno#14325](https://github.com/kyverno/kyverno/issues/14325)
