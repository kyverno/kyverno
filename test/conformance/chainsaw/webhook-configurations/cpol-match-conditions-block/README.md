## Description

This test checks fine-grained webhook configuration is synced to admission webhooks.

## Expected Behavior

The request sent from `system:masters` group should be forwarded to Kyverno and get blocked due to the violation

## Reference Issue(s)

#9111
