## Description

This test creates a cluster policy with a mutation rule containing a foreach and json patch.

## Expected Behavior

No autogen rules should be present in the status as json patches are supposed to disable autogen.

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/4731
