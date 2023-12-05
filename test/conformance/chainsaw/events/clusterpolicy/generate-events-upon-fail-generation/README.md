## Description

This test creates a generate policy, and a resource. A `PolicyError` event should be created upon the failure.

## Steps

1.  - Create a policy
    - Assert the policy becomes ready
1.  - Create a resource
1.  - Asset a `PolicyError` event is created

## Reference Issue(s)

https://github.com/kyverno/kyverno/pull/8466
https://github.com/kyverno/kyverno/pull/1413
