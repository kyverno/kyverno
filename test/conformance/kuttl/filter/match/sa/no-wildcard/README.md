## Description

This test creates a policy, matching service account `system:serviceaccount:kyverno:kyverno`.
This policy denies pod creation.

## Expected Behavior

The pod should be accepted (user is `kubernetes-admin`).

## Related issue(s)

- https://github.com/kyverno/kyverno/issues/7938
