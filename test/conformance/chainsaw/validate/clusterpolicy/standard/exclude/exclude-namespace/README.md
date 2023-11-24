## Description

This test creates a policy to validate all resources have a `foo: bar` label.
The policy matches on a wildcard but excludes a whole Namespace.
The net result should be any Namespaced resource in the excluded Namespace should not be processed.
It then creates a configmap in the default namespace that doesn't have the expected label.


## Expected Behavior

The configmap should be created successfully as it is excluded by the policy.
