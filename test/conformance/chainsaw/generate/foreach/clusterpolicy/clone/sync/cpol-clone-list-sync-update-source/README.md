## Description

This test checks the synchronize behavior for a "generate foreach cloneList" policy upon source changes.

## Expected Behavior

1. trigger the standard policy, expect a secret `foreach-cpol-clone-list-sync-delete-source-target-ns-1/mysecret-1` to be cloned.
2. update the source secret, expect changes to be synced to the cloned secret `foreach-cpol-clone-list-sync-delete-source-target-ns-1/mysecret-1`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542