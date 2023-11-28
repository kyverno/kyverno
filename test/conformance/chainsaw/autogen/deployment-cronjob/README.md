## Description

The policy should contain autogen rules for cronjobs and deployments because it has the `pod-policies.kyverno.io/autogen-controllers: Deployment,CronJob` annotation.

## Expected Behavior

The policy gets created and contains a autogen rules for cronjobs and deployments in the status.

## Related Issue(s)

- https://github.com/kyverno/kyverno/issues/7444
