## Description

This test validate the reporting ability for a audit policy with the `namespaceSelector` defined.

## Expected Behavior

A policy report should be created for the pod `test-audit-reports-namespacesselector/audit-pod`, but not for `test-non-audit-reports-namespacesselector/non-audit-pod` as the namespace selector doesn't match.

