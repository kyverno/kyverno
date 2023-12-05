## Description

This test creates a policy that only allows a maximum of 3 containers inside a pod. It then creates an exception with `conditions` field defined which tests out the functionality for the conditions support in `PolicyException`.


## Expected Behavior

If the exception is not applied, both the deployments `bad-deployment` and `good-deployment` should not be allowed but when the exception has been applied, `good-deployment` should be able to pass through the Policy as it satisfies the conditions mentioned in the `PolicyException`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6223
