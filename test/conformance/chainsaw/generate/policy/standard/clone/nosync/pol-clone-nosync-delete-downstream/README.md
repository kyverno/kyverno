## Description

This test checks to ensure that deletion of a downstream (generated) resource resulting from a Policy (Namespaced) generate rule, clone declaration, with sync disabled, does NOT result the downstream resource's recreation.

## Expected Behavior

The deleted downstream resource should remain deleted. If it is not recreated, the test passes. If it is cloned again from source, the test fails.

## Reference Issue(s)

N/A
