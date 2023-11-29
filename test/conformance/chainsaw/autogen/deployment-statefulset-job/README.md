## Description

The policy should contain autogen rules for deployments, statefulsets and jobs because it has the `pod-policies.kyverno.io/autogen-controllers: Deployment,StatefulSet,Job` annotation.

## Expected Behavior

The policy gets created and contains autogen rules for deployments, statefulsets and jobs in the status.
