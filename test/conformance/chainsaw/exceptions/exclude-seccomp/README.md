## Description

This test creates a policy that enforces the baseline profile and a policy exception that exempts any pod in the `staging-ns` namespace and sets the `spec.securityContext.seccompProfile.type` to `Unconfined`.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace that doesn't violate the baseline profile, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` in the `staging-ns` namespace and the `spec.securityContext.seccompProfile.type` is set to `Unconfined` and the `spec.containers[*].securityContext.seccompProfile.type` is set to `RuntimeDefault`, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` in the `staging-ns` namespace and the `spec.securityContext.seccompProfile.type` is set to `Unconfined` and the `spec.containers[*].securityContext.seccompProfile.type` is set to `Unconfined`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` in the `default` namespace and the `spec.securityContext.seccompProfile.type` is set to `Unconfined` and the `spec.containers[*].securityContext.seccompProfile.type` is set to `Unconfined`, expecting the creation to fail.
