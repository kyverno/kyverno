## Description

This test ensures synchronized `generate.cloneList` cleanup on ClusterPolicy deletion respects `orphanDownstreamOnPolicyDelete`.

## Expected Behavior

1. When `orphanDownstreamOnPolicyDelete: false`, deleting the policy removes generated downstream resources.
2. When `orphanDownstreamOnPolicyDelete: true`, deleting the policy preserves generated downstream resources.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/16168
