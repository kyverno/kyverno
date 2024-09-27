## Description

This test checks the synchronize behavior for a "generate foreach clone" policy upon target changes.

## Expected Behavior

1. trigger the standard policy, expect a secret `foreach-ns-1/cloned-secret-0-foreach-ns-1` to be cloned.
2. update the cloned secret, expect changes to be reverted to the cloned secret `foreach-ns-1/cloned-secret-0-foreach-ns-1`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542