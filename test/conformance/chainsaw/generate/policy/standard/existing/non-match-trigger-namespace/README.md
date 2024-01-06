## Description

This test checks the generateExisting namespaced policy is not applied when the trigger is not found in the same namespace as the policy.

## Expected Behavior

If the resource secret is not created, the test passes. If it is created, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6519