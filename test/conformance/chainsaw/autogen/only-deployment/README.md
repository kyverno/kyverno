## Description

The policy should contain a single autogen rule for deployments because it has the `pod-policies.kyverno.io/autogen-controllers: Deployment` annotation.

## Expected Behavior

The policy gets created and contains a single autogen rule for deployments in the status.
