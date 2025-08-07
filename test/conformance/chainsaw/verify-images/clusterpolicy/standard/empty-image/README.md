## Description

This test creates a policy to verify images signature.
It then creates a `Deployment` that references an image with an empty string.

## Expected Behavior

The deployment should be created without error.
