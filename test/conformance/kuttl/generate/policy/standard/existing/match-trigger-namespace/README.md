## Description

This test checks the generateExisting namespaced policy is applied when the trigger is found in the same namespace as the policy.

## Expected Behavior

If the resource secret is created, the test passes. If it is not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6519