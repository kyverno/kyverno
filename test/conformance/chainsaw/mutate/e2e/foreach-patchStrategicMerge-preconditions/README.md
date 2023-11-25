## Description

This is a migrated test from e2e. It tests that preconditions inside a foreach loop are substituted properly. Preconditions, in this case, use predefined variables from image registries and so this is a secondary aspect to the test.

## Expected Behavior

The containers with images from `docker.io` should be mutated so the value of the `image` field with respect to the registry is replaced with `my-private-registry`. Therefore, the input image `nginx:1.14.2` (which implicitly is equal to `docker.io/nginx:1.14.2`) is mutated so the output is `my-private-registry/nginx:1.14.2`. If this occurs, the test passes. If this is not done, the test fails.

## Reference Issue(s)

N/A