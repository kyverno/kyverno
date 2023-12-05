## Description

This test checks to ensure that deletion of the source (upstream) resource used by a Policy (Namespaced) generate rule, clone declaration, with sync disabled, does NOT result in the downstream resource's deletion.

## Expected Behavior

The deleted downstream resource should remain in place. If it is still present after the source deletion, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

N/A
