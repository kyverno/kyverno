## Description

This test validates that an incoming request to `Node` triggers mutating the existing `Node/status` subresource.

## Expected Behavior

The existing `Node/status` subresource is mutated.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that matches on `Node` and mutates `Node/status` object.
2. Create `ClusterRole` for allowing modifications to `Node/status` subresource.
3. Modify kyverno `resourceFilters` to allow mutating requests for `Node` resource.
4. Send a update request to `Node`.
5. Mutate the existing `Node/status` subresource.
6. Verify that the existing `Node/status` object is mutated.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/2843
