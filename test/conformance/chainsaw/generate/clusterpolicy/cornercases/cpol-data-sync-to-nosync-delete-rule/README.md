## Description

This test checks to ensure that deletion of a rule in a ClusterPolicy generate rule, data declaration, with sync disabled, does not result in the downstream resource's deletion.

## Expected Behavior

The downstream (generated) resource is expected to remain if the corresponding rule within a ClusterPolicy is deleted. If it is not deleted, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

