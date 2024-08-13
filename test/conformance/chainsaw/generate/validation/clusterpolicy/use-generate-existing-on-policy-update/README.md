## Description

This test ensures that the creation of a generate policy that makes use of `spec.generateExistingOnPolicyUpdate` is blocked since it is a deprecated field.

## Expected Behavior

The test passes if the policy creation is blocked, otherwise fails.
