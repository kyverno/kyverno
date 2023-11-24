## Description

This is a migration from e2e. It tests for a combination of the global anchor plus the add-if-not-present anchor in a patchStrategicMerge mutate policy with two rules.

## Expected Behavior

Two tests are conducted. In the first, if a Pod contains an emptyDir volume, it should have an annotation added. In the second, the Pod has a hostPath volume and should also receive an annotation. If either one of these Pods does not have the annotation `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` added the test fails. If this annotation is present, the test passes.

## Reference Issue(s)

N/A