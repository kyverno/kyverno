## Description

This test checks that the `default` field in a context variable should replace nil results in mutateExisting policies.

## Expected Behavior

With the mutateExisting policy, the context variable `podName` will assume the value of `empty` since there is no pod whose name is starting with `good-`, and the pod should get created as preconditions matching as the value of the variable is set to default which is `empty` is equal to  `empty`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7148