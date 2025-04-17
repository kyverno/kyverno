## Description

This test verifies the synchronize behavior of generated resource, if the selected source resources using a matched label selector `allowedToBeCloned: "true"` gets changed, the update should be synchronized with the target resource as well.

## Expected Behavior

This test ensures that update of source resource(ConfigMap) match selected using `allowedToBeCloned:  "true"` label get synchronized with target resource created by a ClusterPolicy `generate.cloneList` rule, otherwise the test fails. 

## Reference Issue(s)

#4930