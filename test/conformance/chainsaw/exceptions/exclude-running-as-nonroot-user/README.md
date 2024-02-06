## Description

This test creates a policy that enforces the restricted profile and a policy exception that exempts any pod whose image is `nginx` in the `staging-ns` namespace and sets the `spec.containers[*].securityContext.runAsUser` field to 0.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace that doesn't violate the restricted profile, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` whose image is `nginx` in the `staging-ns` namespace and the `spec.containers[*].securityContext.runAsUser` is set to 0, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` whose image is `nginx` in the `staging-ns` namespace and the `spec.containers[*].securityContext.runAsUser` is set to 0 and the `spec.initContainers[*].securityContext.runAsNonRoot` is set to 0, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` whose image is `busybox` in the `staging-ns` namespace and the `spec.containers[*].securityContext.runAsUser` is set to 0, expecting the creation to fail.
    - Try to create a pod named `bad-pod-3` whose image is `nginx` in the `default` namespace and the `spec.containers[*].securityContext.runAsUser` is set to 0, expecting the creation to fail.
