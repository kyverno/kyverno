## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync NOT enabled, when the policy is deleted does NOT cause the downstream (generated) resource to also be deleted.

## Expected Behavior

If the resource is retained after the Policy is deleted, the test passes. If it is deleted, the test fails.

## Reference Issue(s)

N/A