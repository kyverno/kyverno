## Description

This test checks to ensure that deletion of a Policy (Namespaced) generate rule, clone declaration, with sync enabled and `orphanDownstreamOnPolicyDelete: true`, does NOT result in the downstream resource's deletion. Without that field (default `false`), deleting the policy removes the downstream.

## Expected Behavior

The downstream (generated) resource is expected to remain if the Policy is deleted. If it is not deleted, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

N/A
