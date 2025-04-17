## Description

This test ensures that the secret is cloned from a source resource has the label `app.kubernetes.io/managed-by: kyverno`.

## Expected Behavior

If the downstream resource is delete after source is deleted, the test passes. If it is not created, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/9718
