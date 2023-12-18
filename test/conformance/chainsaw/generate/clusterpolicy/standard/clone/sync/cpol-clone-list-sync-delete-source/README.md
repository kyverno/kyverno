## Description

This is a corner case test to ensure the corresponding downstream target is deleted when its trigger is deleted, for a generate cloneList type of policy.

## Expected Behavior

If the downstream resources `mysecret-1` and `mysecret-2` are remained in the namespace `cpol-clone-list-sync-delete-source-trigger-ns-2`, the test passes. If not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7535