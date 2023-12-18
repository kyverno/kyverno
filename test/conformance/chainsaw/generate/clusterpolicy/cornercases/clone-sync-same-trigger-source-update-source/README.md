## Description

This is a corner case test to ensure a new downstream target is created when the source matches a different namespace, for a generate clone type of policy. This is a corner case because the source and the trigger is the same resource.

## Expected Behavior

The new downstream resource should be created after the trigger is updated. Otherwise the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7281