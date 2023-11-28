# ## Description

This test validates that the namespaceSelector is applied to a wildcard policy successfully.

## Expected Behavior

The pod `test-validate/nginx-block` is blocked, and the pod `default/nginx-pass` is created.


## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6015
https://github.com/kyverno/kyverno/issues/7771