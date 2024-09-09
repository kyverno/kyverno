## Description

This test checks fine-grained webhook configuration is synced to admission webhooks.

## Expected Behavior

When a policy is created, a webhook rule is automatically created with the same `matchConditions` as configured in the policy. The corresponding webhook rule will be deleted when the policy is deleted.

## Reference Issue(s)

#9111
