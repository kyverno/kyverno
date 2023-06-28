## Description

This test ensures that modification of the downstream (cloned/generated) resource used by a ClusterPolicy `generate` rule with sync enabled using a clone declaration and server-side apply causes those changes to be merged from the state of the upstream/source.

## Expected Behavior

After the downstream resource is modified, the changes should be merged with the clone after synchronization occurs. If the downstream resource is synced with the state of the source resource, and also respects the modifications to other fields, the test passes. If the downstream resource doesn't retain the cloned fields and the directly modified fields, the test fails.

## Reference Issue(s)

N/A
