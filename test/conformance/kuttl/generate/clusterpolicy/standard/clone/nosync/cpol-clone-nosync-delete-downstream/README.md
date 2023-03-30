## Description

Tests that the deletion of a downstream resource created with a generate rule, clone, and no synchronization remains deleted and is not recreated.

## Expected Behavior

The deleted resource is expected to not be recreated. If the downstream resource is regenerated, the test fails. If it is not regenerated, the test succeeds.

## Reference Issue(s)

4457
