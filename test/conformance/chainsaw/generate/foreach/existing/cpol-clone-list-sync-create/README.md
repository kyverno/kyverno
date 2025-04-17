## Description

This test checks the generateExisting behavior for a "generate foreach cloneList" policy upon policy creation.

## Expected Behavior

1. when a policy is created with `generate.generateExisting: true`, expect target netpol `foreach-existing-cpol-clone-list-sync-create-target-ns-1/mysecret-1`to be created.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/3542