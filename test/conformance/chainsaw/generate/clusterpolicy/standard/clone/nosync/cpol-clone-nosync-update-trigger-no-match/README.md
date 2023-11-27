## Description

This test checks to ensure that updates to a trigger which cause it to no longer match the rule, with a generate clone declaration and sync disabled, does not result in the downstream resource's deletion.

## Expected Behavior

If the downstream resource is deleted, the test fails. If it remains, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6507