## Description

This test verifies that resource creation is not blocked if resource image is different than policy image.

## Expected Behavior

This test should create a policy with missing configmap, a pod with different image than policy image. This shouldn't block pod creation.
When pod is created with same image as policy image, pod creation should be blocked.
When test tries to update any field in a policy, it should get updated properly.

## Reference Issue(s)

3709
