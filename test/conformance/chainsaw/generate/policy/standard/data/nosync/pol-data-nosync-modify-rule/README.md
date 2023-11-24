## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync NOT enabled, when the rule definition (under the data object) is modified this does NOT cause those changes to be propagated to downstream (generated) resources.

## Expected Behavior

If the resource is not synced from the changes made to the rule, the test passes. If it is synced, the test fails.

## Reference Issue(s)

N/A