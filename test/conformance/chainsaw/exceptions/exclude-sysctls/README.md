## Description

This test creates a policy that enforces the baseline profile and a policy exception that exempts any pod whose namespace is `staging-ns` namespace and sets the `spec.securityContext.sysctls[*].name` to `fake.value`.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above.
1.  - Try to create a pod named `good-pod-1` in the `default` namespace whose `spec.securityContext.sysctls[0].name` field is set to `net.ipv4.ip_unprivileged_port_start`, expecting the creation to succeed.
    - Try to create a pod named `good-pod-2` in the `staging-ns` namespace whose `spec.securityContext.sysctls[0].name` field is set to `fake.value`, expecting the creation to succeed.
    - Try to create a pod named `bad-pod-1` in the `staging-ns` namespace whose `spec.securityContext.sysctls[0].name` field is set to `unknown`, expecting the creation to fail.
    - Try to create a pod named `bad-pod-2` in the `default` namespace whose `spec.securityContext.sysctls[0].name` field is set to `fake.value`, expecting the creation to fail.
