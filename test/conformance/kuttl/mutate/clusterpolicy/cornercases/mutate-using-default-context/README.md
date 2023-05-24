## Description

This test checks that the `default` field in a context variable should replace nil results in mutateExisting policies.

## Expected Behavior

With the mutateExisting policy, the context variable `tokenvolname` will assume the value of `''` since there is no volume or volumeMounts in the containers inside the pod whose name is starting with `kube-api-access-`, and the pod should get created as a result of being skipped due to preconditions not matching as `''` is not equal to `?*`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7148