## Description

This test verifies the synchronize behavior of generated resource, if the selected source resources using a matched label selector `allowedToBeCloned: "true"` gets changed, the update should be synchronized with the target resource as well.

## Expected Behavior

1. update source resource (configmap) match selected using `allowedToBeCloned:  "true"` label, the change should be synced to the target configmap.
2. remove configmap from the `cloneList.kinds` in the policy, update the source configmap, the change should not be synced to the previous cloned configmap

## Reference Issue(s)

#4930