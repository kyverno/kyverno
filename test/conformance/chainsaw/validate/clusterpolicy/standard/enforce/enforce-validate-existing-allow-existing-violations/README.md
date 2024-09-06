## Description

This test mainly verifies that an enforce validate policy blocks changes in old objects that were present before policy was created when `allowExistingViolations` is set to `false`

## Expected Behavior

1. A bad pod is created that violates the policy.
2. The policy is applied.
3. Violating changes in bad pod causes error becuase `allowExistingViolations` is set to `false`

## Reference Issue(s)

10084
