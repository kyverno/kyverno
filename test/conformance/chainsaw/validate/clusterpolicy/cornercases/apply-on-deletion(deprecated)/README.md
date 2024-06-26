## Description

This test ensures the policy is applied on the resource to be deleted (deletionTimestamp is set).

## Expected Behavior

With a bogus finalizer added to the service, the resource deletion is blocked as no controller serves behind to perform deletion. During this time, when one tries to patch the service that violates the policy, the patch request should be blocked. While if the patch doesn't result in an violation it should be allowed.

## Reference Issue(s)

N/A