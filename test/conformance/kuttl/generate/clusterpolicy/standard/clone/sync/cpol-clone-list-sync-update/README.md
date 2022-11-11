## Description

This test ensures that creation of a multiple target resource created by a ClusterPolicy `generate.cloneList` rule. If it is not generated, the test fails. 

Further it verifies the sync behavior, if the source resource gets changed, the update should be synchronized with the target resource as well.

## Reference Issue(s)

#4930