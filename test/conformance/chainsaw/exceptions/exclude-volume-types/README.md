## Description

This test creates a policy that enforces the restricted profile and a policy exception that exempts any pod whose namespace is `staging-ns` namespace and makes use of `spec.volumes[*].flexVolume`.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace and makes use of `spec.volumes[*].configMap`, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` in the `staging-ns` namespace and makes use of `spec.volumes[*].flexVolume`, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` in the `staging-ns` namespace and makes use of `spec.volumes[*].gcePersistentDisk`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` in the `default` namespace and makes use of `spec.volumes[*].gcePersistentDisk`, expecting the creation to fail.
