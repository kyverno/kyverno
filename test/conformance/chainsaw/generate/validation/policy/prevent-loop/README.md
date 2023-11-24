## Description

This test ensures that a generate policy is allowed to have the same kind defined in the trigger and the target resources. The flag `--backgroundServiceAccountName` was added to prevent endless loop.

## Expected Behavior

The test passes if the policy creation is allowed, otherwise fails.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7280