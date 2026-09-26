## Description

This test tries to create a background ClusterPolicy using a variable that only looks like an allowed
root (`invalid_request.object.metadata.name`). The variable allowlist is anchored, so a name that
merely contains `request` must not be accepted.

## Expected Behavior

The policy should be rejected.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/17283
