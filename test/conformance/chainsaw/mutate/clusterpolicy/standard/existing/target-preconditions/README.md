## Description

This test creates three `ConfigMap`s:
- one without labels
- one with label `foo: bar`
- one with label `foo: not_bar`

It then creates a `ClusterPolicy` with a mutate existing rule targeting the previously created `ConfigMap`s.

The policy rule uses preconditions on the trigger resource to match only `ConfigMap`s with the `trigger` name.
The policy rule also uses preconditions on target resources to match only `ConfigMap`s with he label `foo: bar`.
The policy mutates target resources passing preconditions by copying the `data.content` from the trigger `ConfigMap` to the target `ConfigMap`.

Finally, the test creates the trigger config map.

## Expected Behavior

Only the target config map with label `foo: bar` should have its content updated.