## Description

The `namespaceSelector` should applies to mutateExisting policies upon policy events.

## Expected Behavior
The pod is mutated with annotation `org: kyverno-test`.

## Steps

### Test Steps

1. Create a pod and a configmap in `test` namespace labeled by `org: kyverno-test`.
2. Create a `ClusterPolicy` that mutates existing pod.
4. The pod should be mutated with the annotation `org: kyverno-test`.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6176
