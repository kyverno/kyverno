## Description

This test ensures that deletion of a ClusterPolicy, with a generate rule using clone and sync and `orphanDownstreamOnPolicyDelete: true`, does NOT cause the downstream resource to be deleted. Without that field (default `false`), deleting the policy removes the downstream.

## Expected Behavior

Once the ClusterPolicy is deleted, the downstream resource is expected to remain. If it does remain, the test passes. If it gets deleted, the test fails.

## Reference Issue(s)

N/A