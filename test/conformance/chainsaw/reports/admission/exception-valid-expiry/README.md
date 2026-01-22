# Test: Valid Exception with Future Expiry

This test validates that policy exceptions with a future `expiresAt` date are properly applied.

## Description

When a `PolicyException` has an `expiresAt` field set to a date in the future, the exception should be applied and the policy should be bypassed for matching resources.

## Steps

1. Create a ClusterPolicy that requires a `team` label on ConfigMaps
2. Create a PolicyException with a valid (future) `expiresAt` date (set to 2099-12-31)
3. Attempt to create a ConfigMap without the required label
4. Verify that the ConfigMap creation is allowed (exception is applied)

## Expected Behavior

The ConfigMap creation should succeed because:
- The exception has not expired (expiresAt is in the future)
- The exception is applied during policy evaluation
- The policy enforcement is bypassed for the matching resource
