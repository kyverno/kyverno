## Description

This test validates that an incoming request to namespace is mutated by the mutation and mutated 
the crd of istio operator.

## Expected Behavior

The request is mutated.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that matches on `Namespace` and mutates the request.
2. Create namespace `test-namespace`
3. Mutate the incoming request (done by kyverno).
4. Verify that the request is mutated.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5769
