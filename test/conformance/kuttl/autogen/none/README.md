## Description

The policy should contain no autogen rules because it has the `pod-policies.kyverno.io/autogen-controllers: none` annotation.

## Expected Behavior

The policy gets created and have no autogen rules recorded in the status.
