## Description

This test creates a policy to validate all resources have a `foo: bar` label, excluding the `default` namespace.
It then creates a configmap in the default namespace with the expected label.


## Expected Behavior

The configmap should be created successfully as it is excluded by the policy.
