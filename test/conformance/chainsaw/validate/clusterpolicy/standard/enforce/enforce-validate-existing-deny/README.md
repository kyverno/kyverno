## Description

This test mainly verifies that an enforce validate policy does not block changes in old objects that were present before policy was created

## Expected Behavior

1. A pod is created that violates the policy.
2. The policy is applied.
3. A pod is created that follows the policy.
4. Violating changes on bad pad does not cause error.
5. Violating changes in good pod causes error.
6. The bad pod once passed the policy, will be tracked by the policy and return error on bad changes.
## Reference Issue(s)

8837