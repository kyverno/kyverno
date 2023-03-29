# Title

This test create an image verification policy runing in the background.
It then creates a pod that satisfies the policy.
Note: the pod has to be created first because we don't want the policy to apply at admission time.

## Expected Behavior

The pod is created and a background scan report is generated for it with a pass result.
