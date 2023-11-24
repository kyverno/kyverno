## Description

This test checks to ensure that updates to a trigger which cause it to no longer match the rule, with a generate data declaration and sync enabled, results in the downstream resource's deletion.

## Expected Behavior

If the downstream resource is deleted, the test passes. If it remains, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6507