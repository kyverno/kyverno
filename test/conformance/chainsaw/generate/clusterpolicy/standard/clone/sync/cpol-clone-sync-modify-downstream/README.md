## Description

This test ensures that modification of the downstream (cloned/generated) resource used by a ClusterPolicy `generate` rule with sync enabled using a clone declaration causes those changes to be reverted and synchronized from the state of the upstream/source.

## Expected Behavior

After the downstream resource is modified, the changes should be reverted after synchronization occurs. If the downstream resource is synced with the state of the source resource, the test passes. If the downstream resource remains in a modified state, the test fails.

## Reference Issue(s)

N/A