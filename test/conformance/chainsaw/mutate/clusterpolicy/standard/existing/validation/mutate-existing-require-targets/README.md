## Description

This test ensures that a mutate existing policy has to have `mutate.targets` defined if `mutateExistingOnPolicyUpdate` is true.

## Expected Behavior

With `mutateExistingOnPolicyUpdate` set to true, the policy should be rejected if the `mutate.targets` is not defined, and allowed if `mutate.targets` is specified.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6593