## Description

This test ensures that the `namespaceSelector` applies to mutateExisting policies upon policy events, in this case creation of the ClusterPolicy.

## Expected Behavior

The pod is mutated with annotation `org: kyverno-test`.

## Steps

### Test Steps

1. Create a pod and a configmap in the `org-label-inheritance-existing-ns` namespace labeled by `org: kyverno-test`.
2. Create a `ClusterPolicy` that mutates existing pods.
3. The pod should be mutated with the annotation `org: kyverno-test` present on the parent namespace.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6176
