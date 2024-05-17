## Description

This test verifies that when failurePolicy is set to to Ignore for a policy that was set to Enforce, Admission webhook denies requests when validation of a resource fails. The error should not get consumed by ignore failurePolicy 

## Expected Behavior

The pod should be not created.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8916