## Description

This test checks to ensure that a generate rule with a data declaration and NO synchronization, when the ClusterPolicy is deleted does NOT cause the generated resources to be deleted.

## Expected Behavior

If the downstream resource remains after deletion of the ClusterPolicy, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

N/A