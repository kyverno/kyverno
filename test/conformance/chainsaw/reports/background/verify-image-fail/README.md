# Title

This test creates pods using an unsigned or not correctly signed image.
It then creates an image verification policy running in the background.

Note: the pods have to be created first because we don't want the policy to apply at admission time.

## Expected Behavior

The pods are created and policy reports are generated with a fail result.
