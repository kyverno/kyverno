## Description

This is a corner case test to ensure the corresponding downstream target is deleted when its source is deleted, for a generate cloneList type of policy. This is a corner case because the source and the trigger is the same resource.

## Expected Behavior

If the downstream resource `mysecret-1` is deleted while `mysecret-2` remains, the test passes. If not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7535