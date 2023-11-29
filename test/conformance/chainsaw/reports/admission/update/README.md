## Description

This test verifies that policy report is correctly updated when a resource changes.
A policy in Audit mode is created.
A deployment is created, the deployment violates the policy and we assert the policy report contains a `fail` result.
The deployment is then updated to not violate the policy anymore and we assert the policy report changes to contain `pass` result.

## Expected result

When the resource does not violate the policy anymore, the result in the policy report should change from `fail` to `pass`.

## Related issue(s)

- https://github.com/kyverno/kyverno/issues/7793