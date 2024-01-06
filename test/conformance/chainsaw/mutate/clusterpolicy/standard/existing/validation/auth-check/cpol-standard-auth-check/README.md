## Description

This test ensures that a mutate existing policy is denied when it does not have corresponding permissions to generate the downstream resource.

## Expected Behavior

The test fails if the policy creation is allowed, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6584