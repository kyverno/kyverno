## Description

This is a migrated test from e2e. The global anchor is being checked for basic functionality here.

## Expected Behavior

If a container uses an image named `someimagename` then the `imagePullSecret` must be set to `my-registry-secret`. The test passes if this combination is found. If an image named `someimagename` uses some other imagePullSecret, the test fails.

## Reference Issue(s)

2390
