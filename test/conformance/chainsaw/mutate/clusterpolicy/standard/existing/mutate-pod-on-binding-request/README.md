## Description

This test validates that an incoming request to `Pod/binding` subresource can act as a trigger for mutation of an existing `Pod` object.

## Expected Behavior

The `Pod` `nginx-pod` is labelled with `foo: empty` label.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that matches on `Pod/binding` and mutates `Pod` object.
2. Create `ClusterRole` for allowing modifications to `Pod` resource.
3. Modify kyverno `resourceFilters` to allow mutating incoming requests to `Pod/binding` subresource.
4. Modify kyverno `resourceFilters` to allow mutating incoming requests from `kube-system` namespace.
5. Create a `Pod` object.
6. Verify that the `Pod` object is labelled with `foo: empty` label.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6503