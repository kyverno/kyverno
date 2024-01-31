## Description

This is a generate test to ensure deleting a generate policy using a data declaration with sync enabled, orphanDownstreamOnPolicyDelete preserves the downstream ConfigMap.

## Expected Behavior

If the generated configmap is retained, the test passes. If it is not, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9578