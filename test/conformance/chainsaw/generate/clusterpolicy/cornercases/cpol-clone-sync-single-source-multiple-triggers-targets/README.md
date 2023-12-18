## Description

This is a corner case test to ensure the changes to the clone source can be synced to multiple targets.

## Expected Behavior

If the change from `foo=bar` to `foo=baz` is synced to downstream targets, the test passes. Otherwise fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7170