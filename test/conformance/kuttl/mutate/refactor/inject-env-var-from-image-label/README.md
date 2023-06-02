## Description

This is a test of the policy in this folder.

Note: In order for this test to work on Pods emitted from Pod controllers, the Kyverno ConfigMap excludeGroups value may need to be modified to remove the entry for system:serviceaccounts:kube-system or else mutation may not occur.

## Expected Behavior

The resource is expected to be mutated so it resembles the specified asserted resource. If it does, the test passes. If it does not, it fails.

## Reference Issue(s)

N/A