## Description

This test checks to ensure that deletion of a trigger resource, with a generate data declaration and sync enabled, results in its corresponding downstream resource's deletion.

## Expected Behavior

If the downstream resource `foosource-1-replicated` is deleted while the other two `foosource-2-replicated` and `foosource-3-replicated` remain, the test passes. If not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7535