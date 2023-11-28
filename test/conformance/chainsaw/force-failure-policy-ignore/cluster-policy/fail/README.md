## Description

This test creates a policy with `failurePolicy: Fail` but the configuration has `forceWebhookFailurePolicyIgnore: true`.

## Expected Behavior

Webhooks should be configured with `failurePolicy: Ignore` regardless of the failure policy configured in the policies.
