## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync enabled, when the downstream (generated) resource is deleted causes it to be recreated with the definition of the resource stored in the rule.

## Expected Behavior

If the resource is re-created according to the definition in the rule, the test passes. If it remains in a deleted state, the test fails.

## Reference Issue(s)

N/A
