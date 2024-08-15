## Description

This test ensures that creation of a multiple target resource created by a ClusterPolicy `generate.cloneList` rule. If it is not generated, the test fails.

## Expected Behavior

The cloned Secret and ConfigMap from the default namespace should exists in newly created namespace.

## Reference Issue(s)

N/A