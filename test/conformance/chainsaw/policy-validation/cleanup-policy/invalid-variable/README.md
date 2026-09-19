## Description

This test tries to create a ClusterCleanupPolicy whose condition uses an undeclared variable
(`mytarget.metadata.name`) that only looks like the allowed `target` root. The variable allowlist is
anchored, so an undeclared context variable must not be accepted.

## Expected Behavior

The policy should be rejected.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/17283
