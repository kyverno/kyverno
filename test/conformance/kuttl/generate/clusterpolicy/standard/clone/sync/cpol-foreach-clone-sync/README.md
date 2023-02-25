## Description

This test checks to ensure that deletion of a rule in a ClusterPolicy generate rule with sync enabled, does result in the downstream resource's deletion.

## Expected Behavior

The downstream (generated) resource is expected to delete if the corresponding rule within a ClusterPolicy is deleted. If it is not deleted, the test fails. If it is deleted, the test passes.

## Reference Issue(s)

N/A
