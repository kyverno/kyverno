## Description

This is a corner case test to ensure the downstream target is deleted when the source is deleted, for a generate cloneList type of policy. This is a corner case because the source and the trigger is the same resource.

## Expected Behavior

If the downstream resource is deleted, the test passes. If not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7281