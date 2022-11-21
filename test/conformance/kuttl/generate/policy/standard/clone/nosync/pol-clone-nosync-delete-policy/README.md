## Description

This test checks to ensure that deletion of a Policy (Namespaced) generate rule, clone declaration, with sync disabled, does NOT result in the downstream resource's deletion.

## Expected Behavior

The downstream (generated) resource is expected to remain if the Policy is deleted. If it is not deleted, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

N/A
