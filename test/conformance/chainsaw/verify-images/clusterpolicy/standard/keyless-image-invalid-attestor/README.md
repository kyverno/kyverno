## Description

Verify image signature with the given predicateType and attestors. The rule has multiple attestors with an invalid attestor

## Expected Behavior

The rule has two attestors, first attestor is invalid, second attestor is valid. Since count is set to 1, the pod creation should pass.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8842
