## Description

Verify image attestations with the given predicateType and attestors. The image has multiple signatures for different predicateTypes.

## Expected Behavior

Given the defined predicateType, the matching attestor entries must greater than or equal to the count specified in the rule. This test has one valid attestor so the pod creation should be allowed.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/4847
