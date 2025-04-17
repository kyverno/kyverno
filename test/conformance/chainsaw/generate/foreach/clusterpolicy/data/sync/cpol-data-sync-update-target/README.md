## Description

This test checks the synchronize behavior for a "generate foreach data" policy upon target changes.

## Expected Behavior

1. create the standard policy, expect a netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1` to be created.
2. change the target resource, expect changes in netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1-new` to be reverted.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542