## Description

This test ensures that deletion of a ClusterPolicy, with a generate rule using clone and sync, does NOT cause the downstream resource to be deleted.

## Expected Behavior

Once the ClusterPolicy is deleted, the downstream resource is expected to remain. If it does remain, the test passes. If it gets deleted, the test fails.

## Reference Issue(s)

N/A