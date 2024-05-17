## Description

This tests that the ownerReferences of cloned objects in different Namespaces are removed. Otherwise these objects will be immediately garbage-collected

## Expected Behavior

The background controller will strip the ownerReference when cloning between Namespaces, if it exists.

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/2276
