## Description

The policy should contain a single autogen rule for cronjobs because it has the `pod-policies.kyverno.io/autogen-controllers: CronJob` annotation.

## Expected Behavior

The policy gets created and contains a single autogen rule for cronjobs in the status.
