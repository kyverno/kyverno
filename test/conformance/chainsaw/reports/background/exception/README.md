## Description

This test creates a policy, a policy exception and a configmap.
It makes sure the generated background scan report contains a skipped result instead of a failed one.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above, configured to apply to configmap named `emergency`
1.  - Try to create a confimap named `emergency`
1.  - Assert that a policy report exists with a skipped result
