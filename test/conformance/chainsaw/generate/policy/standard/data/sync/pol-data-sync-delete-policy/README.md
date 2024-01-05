## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync enabled, when the policy is deleted causes the downstream (generated) resource to also be deleted.

## Expected Behavior

If the resource is deleted after the Policy is deleted, the test passes. If it remains, the test fails.

## Reference Issue(s)

5753