## Description

This test creates the following global context entries with projections:
1. A projection with both `jmesPath` and `expression` set.
2. A projection with neither `jmesPath` nor `expression` set.
3. A projection with only `expression` set.

## Expected Behavior

1st and 2nd global context entries should be rejected by the `x-kubernetes-validations` rule on
`GlobalContextEntryProjection`. The 3rd entry should get created.
