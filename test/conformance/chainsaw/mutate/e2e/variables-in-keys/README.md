## Description

This is a migrated test from e2e. It tests that variable substitution is occurring properly in the key of a patchStrategicMerge rule.

## Expected Behavior

The annotation `fluentbit.io/exclude-busybox: "true"` is expected to be written to the Deployment. If it is, the test passes. If it is not, the test fails.

## Reference Issue(s)

N/A