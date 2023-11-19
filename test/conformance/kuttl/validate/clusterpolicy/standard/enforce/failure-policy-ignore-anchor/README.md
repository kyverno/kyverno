## Description

This test verifies that when failurePolicy is set to to Ignore for a policy that was set to Enforce, Admission webhook does not deny requests when validation of a resource fails.

## Expected Behavior

The pod should be created.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8916