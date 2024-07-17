## Description

This test ensures that request.oldObject is not null on UPDATE operations when there are multiple rules in a policy.

## Expected Behavior

The namespace update operation is allowed.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9885