## Description

Verify image attestations with the given predicateType and attestors. The image has multiple signatures for different predicateTypes.

## Expected Behavior

Given the defined predicateType, all attestor entries must be valid if the count is not specified. This test only has one valid attestor so the pod creation should be blocked.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/4847
