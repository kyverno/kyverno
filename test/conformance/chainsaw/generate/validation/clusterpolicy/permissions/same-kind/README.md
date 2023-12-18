## Description

This test ensures that the auth checks for generate policy is performed against given the APIVersion and the subresource.

## Expected Behavior

The test fails if the policy that generates `k8s.nginx.org/v1/policy` and its subresource can be created, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7618