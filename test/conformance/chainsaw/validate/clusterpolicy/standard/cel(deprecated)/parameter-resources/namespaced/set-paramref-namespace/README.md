## Description

This test validates the use of parameter resources in validate.cel subrule.

This test creates the following:
1. A namespace `test-params`
2. A namespaced custom resource definition `ReplicaLimit`
3. A policy that checks the deployment replicas using the parameter resource. The `validate.cel.paramRef.namespace` is set.
4. Two deployments.

## Expected Behavior

The deployment `deployment-fail` is blocked, and the deployment `deployment-pass` is created.
