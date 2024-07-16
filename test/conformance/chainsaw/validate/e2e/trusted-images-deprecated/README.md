## Description

This test is migrated from e2e. It tests an imageRegistry context lookup for a "real" image and states that an image built to run as root can only come from GHCR.

## Expected Behavior

If an image is built to run as root user and it does NOT come from GHCR, the Pod is blocked. If it either isn't built to run as root OR it is built to run as root and does come from GHCR, it is allowed.

## Reference Issue(s)

N/A