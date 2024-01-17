## Description

This test validates the use of parameter resources in validate.cel subrule.

This test creates the following:
1. A namespace `test-params`
2. A namespaced custom resource definition `ReplicaLimit`
3. A policy that checks the statefulset replicas using the parameter resource. The `validate.cel.paramRef.namespace` is unset so it is expected to retrieve the parameter resource from the statefulset's namespace
4. Two statefulsets.

## Expected Behavior

The statefulset `statefulset-fail` is blocked, and the statefulset `statefulset-pass` is created.
