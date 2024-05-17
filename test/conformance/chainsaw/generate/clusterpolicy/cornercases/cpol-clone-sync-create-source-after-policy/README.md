## Description

This is a corner case test to ensure a clone rule is applied when the source is created after the ClusterPolicy.

## Expected Behavior

If the downstream resource is created, the test passes. If it is not created, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5411