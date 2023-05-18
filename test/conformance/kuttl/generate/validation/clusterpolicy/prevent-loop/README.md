## Description

This test ensures that a generate policy cannot have the same kind defined in the trigger and the target resources. Otherwise it would result in an endless loop.

## Expected Behavior

The test fails if the policy creation is allowed, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7017