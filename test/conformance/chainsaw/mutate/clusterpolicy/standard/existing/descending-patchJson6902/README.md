## Description

This test creates two `Pod`s: trigger and target. The policy updates the image of the third container in the target pod whemn the trigger pod is created.
## Expected Behavior

When the trigger pod is applied the, image in container3 of target pod changes
