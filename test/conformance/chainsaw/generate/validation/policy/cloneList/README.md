## Description

This test validate clone sources scopes and the namespace settings.

## Expected Behavior

These tests checks:
1. the mixed scoped of clone sources cannot be defined
2. a namespace policy cannot clone a cluster-wide resource
3. the clone source namespace must be set for a namespaced policy


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/7801