## Description

This test creates a policy that uses CEL expressions to check if the statefulset is created in the `production` namespace or not.

## Expected Behavior

The statefulset `bad-statefulset` is blocked, and the statefulset `good-statefulset` is created.
