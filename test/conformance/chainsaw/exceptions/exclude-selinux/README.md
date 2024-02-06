## Description

This test creates a policy that enforces the baseline profile and a policy exception that exempts any pod whose image is `nginx` in the `staging-ns` namespace and fields:
1. `spec.containers[*].securityContext.seLinuxOptions.type` is set to `foo`.
2. `spec.initContainers[*].securityContext.seLinuxOptions.type` is set to `bar`.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace that doesn't violate the baseline profile, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.seLinuxOptions.type` set to `foo` and `spec.initContainers[*].securityContext.seLinuxOptions.type` set to `bar`, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.seLinuxOptions.type` set to `bar` and `spec.initContainers[*].securityContext.seLinuxOptions.type` set to `foo`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` whose image is `busybox` in the `staging-ns` namespace with `spec.containers[*].securityContext.seLinuxOptions.type` set to `foo` and `spec.initContainers[*].securityContext.seLinuxOptions.type` set to `bar`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-3` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.seLinuxOptions.type` set to `foo` and `spec.ephemeralContainers[*].securityContext.capabilities.add` set to `bar`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-4` whose image is `nginx` in the `default` namespace with `spec.containers[*].securityContext.seLinuxOptions.type` set to `foo` and `spec.initContainers[*].securityContext.seLinuxOptions.type` set to `bar`, expecting the creation to fail.
