## Description

This is a test migrated from e2e. It tests that simple conditional anchors (multiple) are working properly using a patchStrategicMerge mutation rule.

## Expected Behavior

For a Pod with only `containers[]`, the `securityContext.runAsNonRoot=true` should be written to each container as well as to the `spec`. For a Pod with an added `initContainers[]` entry, the same should occur for the initContainer as well. If both of these happen as expected, the test passes. If any one does not, the test fails.

## Reference Issue(s)

N/A
