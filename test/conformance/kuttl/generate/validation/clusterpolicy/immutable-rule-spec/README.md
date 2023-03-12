## Description

This test ensures that modification of the rule spec fields defined in a generate ClusterPolicy is disallowed except `spec.generate.synchronize`.

## Expected Behavior

The test fails if the modification is allowed, otherwise passes.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6440