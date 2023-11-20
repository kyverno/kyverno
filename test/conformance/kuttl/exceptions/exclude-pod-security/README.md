## Description

This test creates a policy that enforces the restricted profile and a policy exception that exempts containers running either the nginx or redis image from the Capabilities control.
The policy exception is configured to apply only to the pods that in `staging-ns` namespace.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `goodpod01` whose image is `nginx` in the `staging-ns` namespace that violates the policy, expecting the creation to succeed
    - Try to create a pod named `badpod01` whose image is `nginx` in the `default` namespace that violates the policy, expecting the creation to fail
    - Try to create a pod named `badpod02`  whose image is `busybox` in the `staging-ns` namespace that violates the policy,, expecting the creation to fail
