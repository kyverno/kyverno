## Description

This test creates a policy that enforces the baseline profile and a policy exception that exempts any pod whose namespace is `staging-ns` and make use of the HostPath volume.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace and doesn't use the HostPath volume, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` in the `staging-ns` namespace that uses the HostPath volume, expecting the creation to succeed.
    - Try to create a pod named `bad-pod` in the `default` namespace that makes use of the HostPath volume, expecting the creation to fail.
