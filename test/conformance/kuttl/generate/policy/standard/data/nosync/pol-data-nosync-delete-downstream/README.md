## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync NOT enabled, when the downstream (generated) resource is deleted causes it to NOT be recreated.

## Expected Behavior

If the resource remains in a deleted state, the test passes. If it remains is recreated according to the definition in the rule, the test fails.

## Reference Issue(s)

N/A
