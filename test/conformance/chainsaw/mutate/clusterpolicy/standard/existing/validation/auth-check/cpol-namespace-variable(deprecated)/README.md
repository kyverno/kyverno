## Description

This test ensures that a mutate existing policy is denied when the target has the namespace defined as variable.

## Expected Behavior

The test fails if the policy creation is allowed, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7213