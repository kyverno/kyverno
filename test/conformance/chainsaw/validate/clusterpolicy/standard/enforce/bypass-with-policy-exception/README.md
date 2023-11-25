# ## Description

This test validates that a policy blocking scaling using `Deployment/scale` resource can be bypassed using `PolicyException`.

## Expected Behavior

The `Deployment` is scaled.

## Steps

### Test Steps

1. Create a `ClusterPolicy` that matches on `Deployment/scale` and blocks scaling the `Deployment`.
2. Create a `Deployment` with the number of replicas allowed in the policy.
3. Create a `PolicyException` for the above mentioned policy.
4. Validate that the `Deployment` is scaled.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5804