## Description

This test checks the generate rule to be applied on Kyverno generated resources when `skipBackgroundRequests` is disabled.

## Expected Behavior

The serviceaccount is created when Kyverno creates a new secret.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9131
