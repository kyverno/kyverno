## Description

This is a migrated test from e2e. It checks that the global anchor works in tandem with a patchStrategicMerge policy.

## Expected Behavior

If a container image is prefaced with `registry.corp.com` then it should be mutated to add an imagePullSecret named `regcred`. If this is done, the test passes. If this is not, the test fails.

## Reference Issue(s)

N/A