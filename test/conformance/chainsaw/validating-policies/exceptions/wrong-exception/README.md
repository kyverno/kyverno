## Description

The policyexceptions should only apply to policies specified and not on all the policies of same kind.

## Expected Behavior

The policyexception is set for a different policy and not the applied policy.

The good-deployment should pass without any failure.
The skipped-deployment should fail regardless of the fact it is added in the wrongly configured policyexception.