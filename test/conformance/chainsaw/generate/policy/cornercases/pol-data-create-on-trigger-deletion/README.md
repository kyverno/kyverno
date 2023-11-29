## Description

This is a corner case test to ensure a generate data rule, with sync enabled, can be triggered on the deletion of the trigger resource.

## Expected Behavior

If the downstream resource is created, the test passes. If it is not created, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6398