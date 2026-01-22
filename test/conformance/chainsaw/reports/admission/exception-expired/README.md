# Test: Exception Expiration

This test validates that expired policy exceptions are properly ignored during policy evaluation.

## Description

When a `PolicyException` has an `expiresAt` field set to a date in the past, the exception should be ignored and the policy should be enforced as normal.

## Steps

1. Create a ClusterPolicy that requires a `team` label on ConfigMaps
2. Create a PolicyException with an expired `expiresAt` date (set to 2020-01-01)
3. Attempt to create a ConfigMap without the required label
4. Verify that the ConfigMap creation is blocked (exception not applied because it's expired)

## Expected Behavior

The ConfigMap creation should fail because:
- The exception has expired (expiresAt is in the past)
- The expired exception is ignored during policy evaluation
- The policy enforces the label requirement
