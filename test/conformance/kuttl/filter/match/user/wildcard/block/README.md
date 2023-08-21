## Description

This test creates a policy, matching users with wildcard `?*`.
This policy denies pod creation.

## Expected Behavior

The pod should be denied (user is `kubernetes-admin`).

## Related issue(s)

- https://github.com/kyverno/kyverno/issues/7938
