## Description

This test validate cloneList sources scopes and the namespace settings.

## Expected Behavior

These tests checks:
1. the mixed scoped of clone sources cannot be defined
2. the namespace must be set if clone namespaced resources
3. the namespace must not be set if clone cluster-wide resources


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7801