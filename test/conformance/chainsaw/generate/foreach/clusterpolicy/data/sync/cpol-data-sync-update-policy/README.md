## Description

This test checks the synchronize behavior for a "generate foreach data" policy upon policy changes.

## Expected Behavior

1. create the standard policy, expect a netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1` to be created.
2. change the target name in `spec.rules.generate.foreach.name`, expect a new netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1-new` to be created.
3. change the data block in `spec.rules.generate.foreach.data`, expect the above netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1-new` to be updated.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542