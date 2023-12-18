# Title

This test creates a pod using a valid signed image.
It then creates an image verification policy running in the background.

Note: the pod has to be created first because we don't want the policy to apply at admission time.

## Expected Behavior

The pod is created and a policy report is generated for it with a pass result.
