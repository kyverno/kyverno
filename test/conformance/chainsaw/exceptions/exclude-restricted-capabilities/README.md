## Description

This test creates a policy that enforces the restricted profile and a policy exception that exempts any pod whose image is `nginx` in the `staging-ns` namespace and fields:
1. `spec.containers[*].securityContext.capabilities.add` is set to `foo`.
2. `spec.initContainers[*].securityContext.capabilities.add` is set to `baz`.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace with `spec.containers[*].securityContext.capabilities.add` set to `NET_BIND_SERVICE`, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.capabilities.add` set to `foo` and `spec.initContainers[*].securityContext.capabilities.add` set to `baz`, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.capabilities.add` set to `baz` and `spec.initContainers[*].securityContext.capabilities.add` set to `foo`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` whose image is `busybox` in the `staging-ns` namespace with `spec.containers[*].securityContext.capabilities.add` set to `foo` and `spec.initContainers[*].securityContext.capabilities.add` set to `baz`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-3` whose image is `nginx` in the `staging-ns` namespace with `spec.containers[*].securityContext.capabilities.add` set to `foo` and `spec.ephemeralContainers[*].securityContext.capabilities.add` set to `baz`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-4` whose image is `nginx` in the `default` namespace with `spec.containers[*].securityContext.capabilities.add` set to `foo` and `spec.initContainers[*].securityContext.capabilities.add` set to `baz`, expecting the creation to fail.
