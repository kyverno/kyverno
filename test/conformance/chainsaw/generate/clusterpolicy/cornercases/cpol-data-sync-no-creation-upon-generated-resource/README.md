## Description

This test checks the generate rule doesn't apply on Kyverno generated resources when `skipBackgroundRequests` is enabled.

## Expected Behavior

The serviceaccount is not created when Kyverno creates a new secret.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9131
