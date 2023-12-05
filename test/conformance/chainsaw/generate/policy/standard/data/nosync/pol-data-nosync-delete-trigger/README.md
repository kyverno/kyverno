## Description

This test checks to ensure that deletion of a trigger resource, with a generate data declaration and sync disabled, doesn't result in the downstream resource's deletion.

## Expected Behavior

If the downstream resource is deleted, the test fails. If it remains, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/2229