## Description

This test ensures the corresponding downstream target is deleted when its trigger is deleted, for a generate foreach cloneList type of policy.

## Expected Behavior

If the downstream resources `mysecret-1` is remained in the namespace `cpol-clone-list-sync-delete-source-trigger-ns-1`, the test fails. If not, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542