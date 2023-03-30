## Description

This test checks to ensure that modification of a downstream (generated) resource resulting from a Policy (Namespaced) generate rule, clone declaration, with sync disabled, does NOT result in those modifications being reverted with the contents of the source resource.

## Expected Behavior

The downstream resource, once modified, should remain as-is. If it remains as-is based on the last modification, the test passes. If it is anything else than how it was last modified, the test fails.

## Reference Issue(s)

N/A
