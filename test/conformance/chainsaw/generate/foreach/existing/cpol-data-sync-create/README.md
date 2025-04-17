## Description

This test checks the generateExisting behavior for a "generate foreach data" policy upon policy creation.

## Expected Behavior

1. when a policy is created with `generate.generateExisting: true`, expect target netpol `foreach-ns-1/my-networkpolicy-0-foreach-ns-1`to be created.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542