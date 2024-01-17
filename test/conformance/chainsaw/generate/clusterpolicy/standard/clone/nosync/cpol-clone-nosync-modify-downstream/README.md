## Description

This test ensures that modification of a downstream (generated) resource, using a generate policy with clone and no-sync, does NOT cause changes to be synchronized downstream.

## Expected Behavior

Once the downstream resource is modified, the downstream resource is expected to remain as-is. If it does remain as-is, the test passes. If the changes get reverted (synced), the test fails.

## Reference Issue(s)

N/A