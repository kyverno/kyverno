## Description

This test checks the synchronize behavior for a "generate foreach cloneList" policy upon target changes.

## Expected Behavior

1. trigger the standard policy, expect a secret `foreach-cpol-clone-list-sync-delete-source-target-ns-1/mysecret-1` to be cloned.
2. update the target cloned secret, expect changes to be reverted to the cloned secret `foreach-cpol-clone-list-sync-delete-source-target-ns-1/mysecret-1`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542