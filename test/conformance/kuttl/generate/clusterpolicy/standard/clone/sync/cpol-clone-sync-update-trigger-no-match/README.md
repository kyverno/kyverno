## Description

This test checks to ensure that updates a trigger that no longer matching the rule, with a generate clone declaration and sync enabled, results in the downstream resource's deletion.

## Expected Behavior

If the downstream resource is deleted, the test passes. If it remains, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6507