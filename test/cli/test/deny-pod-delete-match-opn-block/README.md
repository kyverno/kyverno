## Description

This test checks to ensure that a pod cannot be deleted when the operation is specified in the match block.

## Expected Behavior

If the downstream resource is deleted, the test fails. If it remains, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8644
