## Description

This is a migrated test from e2e. It checks that simple JSON patches function properly when mutating array slices.

## Expected Behavior

If the Pod has a second environment variable added with the name `K8S_IMAGE` with value equal to `busybox:1.11` then the test succeeds. If it does not, the test fails. Note that there is an initContainer present which based upon the policy definition should NOT be mutated.

## Reference Issue(s)

N/A