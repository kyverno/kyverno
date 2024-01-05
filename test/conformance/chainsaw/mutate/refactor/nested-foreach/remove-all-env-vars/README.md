## Description

This test uses a nested foreach to remove all env variables from all containers.

## Expected Behavior

The created pod contains the same containers as the original pod but all env variables in all containers have been removed.

## Reference Issue(s)

5661