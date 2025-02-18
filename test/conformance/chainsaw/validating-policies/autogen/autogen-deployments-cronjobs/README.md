## Description

The policy should contain autogen rules for cronjobs and deployments because it has the `pod-policies.kyverno.io/autogen-controllers: deployments,cronjobs` annotation.

## Expected Behavior

The policy gets created and contains autogen rules for cronjobs and deployments in the status.

