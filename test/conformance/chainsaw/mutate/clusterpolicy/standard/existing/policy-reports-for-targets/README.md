## Description

This test validates that policy reports are correctly created for target resources (not trigger resources) when mutation policies are applied to existing resources. This addresses the issue where reports were being generated for trigger resources instead of the actual resources that were mutated.

## Expected Behavior

When the `trigger-configmap` ConfigMap is created, this should result in:
1. The mutation of the Secret named `target-secret` within the same Namespace to add the label `app: mutated`
2. A policy report being created for the `target-secret` (not for the `trigger-configmap`)
3. The policy report should reference the target resource that was actually mutated

If policy reports are created for the target resource (Secret) and not the trigger resource (ConfigMap), the test passes.

## Reference Issue(s)

- Fixes issue where policy reports were created for trigger resources instead of target resources
- Related to PR #13339: fix policy reports for mutation existing resources 