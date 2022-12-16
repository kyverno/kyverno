## Description

This test validates that an incoming request to `Node/status` is mutated by the mutation policy matching
on `Node/status`.

## Expected Behavior

The request is mutated.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that matches on `Node/status` and mutates the request.
2. Modify kyverno `resourceFilters` to allow mutating requests for `Node` resource.
3. Send a update request to `Node/status`.
4. Mutate the incoming request (done by kyverno).
5. Verify that the request is mutated.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/2843
