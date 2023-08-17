## Description

This test ensures that the target namespace must be set for namespace-scoped target resource, and must not be set for cluster-wide target resources.

## Expected Behavior

The test fails if the policy creation is allowed, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7038
https://github.com/kyverno/kyverno/issues/7470
https://github.com/kyverno/kyverno/issues/7750