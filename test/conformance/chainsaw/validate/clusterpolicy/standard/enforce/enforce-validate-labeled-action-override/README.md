## Description

This test verifies that the `validateFailureActionOverrides` works with a namespaceSelector defined.

## Expected Behavior

1. A namespace with a `kyverno-audit: true` label is created.
2. A policy with a `validationFailureActionOverrides` with said label is created.
3. A bad pod is created in the default namespace and gets blocked.
4. A bad pod is created in the labeled namespace and is allowed.

## Reference Issue(s)

11601
