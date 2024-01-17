## Description

This test tries to create a policy with invalid an invalid subject kind (`Foo`).
Only kinds supported are `User`, `Group`, or `ServiceAccount`.

## Expected Behavior

Policy should be rejected.

## Related Issue

https://github.com/kyverno/kyverno/issues/7052