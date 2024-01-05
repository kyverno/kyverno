## Description

This test ensures that modification of a source (upstream) resource, using a generate policy with clone and no-sync, does NOT cause changes to be synchronized downstream.

## Expected Behavior

Once the upstream resource is modified, the downstream resource is expected to remain as it was prior to the upstream modification. If it does remain, the test passes. If it gets modified (sync), the test fails.

## Reference Issue(s)

N/A