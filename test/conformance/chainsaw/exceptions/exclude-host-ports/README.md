## Description

This test creates a policy that enforces the baseline profile and a policy exception that exempts any pod whose image is `nginx` and hostPort set to either 10 or 20.
The policy exception is configured to apply only to the pods that in `staging-ns-3` namespace.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace whose hostPort is set to zero, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` in the `staging-ns-3` namespace that uses the HostPort control whose values are 10 and 20, expecting the creation to succeed.
    - Try to create a pod named `bad-pod` in the `default` namespace that uses both the HostProcess controls with value 20, expecting the creation to fail.
