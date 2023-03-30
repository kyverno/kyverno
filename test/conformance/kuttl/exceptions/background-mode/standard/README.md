## Description

This test creates policy exceptions with the `spec.background` field. It tests the usage of using components not available in background scans in exceptions.

## Expected Behavior

The polex-right is expected to be created but the polex-wrong should fail due to having a component that isn't available in background scan.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5949