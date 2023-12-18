## Description

This test ensures that deletion of a source (upstream) resource, using a generate policy with clone and no-sync, does NOT cause the downstream resource to be deleted.

## Expected Behavior

Once the upstream resource is deleted, the downstream resource is expected to remain. If it does remain, the test passes. If it gets deleted, the test fails.

## Reference Issue(s)

N/A