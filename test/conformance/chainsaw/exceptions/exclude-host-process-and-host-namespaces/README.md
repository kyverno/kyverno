## Description

This test creates a policy that enforces the baseline profile and exempts any pod that violates the Host Namespaces control and a policy exception that exempts any pod that violates the HostProcess control.
The policy exception is configured to apply only to the pods that in `staging-ns-1` namespace.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `goodpod-01` in the `staging-ns-1` namespace that uses both the Host Namespace and the HostProcess controls, expecting the creation to succeed.
    - Try to create a pod named `goodpod-02` in the `staging-ns-1` namespace that uses the HostProcess control, expecting the creation to succeed.
    - Try to create a pod named `goodpod-03` in the `default` namespace that uses the Host Namespace control, expecting the creation to succeed.
    - Try to create a pod named `badpod-01` in the `default` namespace that uses both the Host Namespace and the HostProcess controls, expecting the creation to fail.
