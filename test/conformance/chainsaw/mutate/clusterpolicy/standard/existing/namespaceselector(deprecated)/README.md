## Description

The `namespaceSelector` should applies to mutateExisting policies upon admission reviews.

## Expected Behavior
The pod is mutated with annotation `org: kyverno-test`.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that mutates existing pod upon configmap operations in namespaces with label `org`.
2. Create a pod in `test` namespace labeled by `org: kyverno-test`.
3. Create a configmap in `test` namespace.
4. The pod should be mutated with the annotation `org: kyverno-test`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6176
