## Description

This test makes sure a mutate existing rule does not patch targets that match the configured `resourceFilters`. With the default `excludeKyvernoNamespace: true`, the `kyverno` namespace is filtered, so resources in it can never trigger policies and must not be mutated as targets either.

## Expected Behavior

When the `trigger` ConfigMap is created in the `filtered-target` Namespace, the `mutate-existing-target` ConfigMap in the same Namespace gets the label `foo: bar`. The `mutate-existing-target` ConfigMap in the `kyverno` Namespace is resolved by the same rule but must be left unchanged.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/18259
