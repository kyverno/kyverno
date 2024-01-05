## Description

This test creates a policy, excluding users with wildcard `?*`.
This policy denies pod creation.

## Expected Behavior

The pod should be accepted (user is `kubernetes-admin`).

## Related issue(s)

- https://github.com/kyverno/kyverno/issues/7938
