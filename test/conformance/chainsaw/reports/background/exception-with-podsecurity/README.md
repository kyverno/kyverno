## Description

This test creates a policy, a policy exception and a pod.
It makes sure the generated background scan report contains a skipped result instead of a failed one.

## Steps

1.  - Create a pod named `nginx`
2.  - Create a cluster policy
    - Assert the policy becomes ready
3.  - Create a policy exception for the cluster policy created above, configured to apply to pod named `nginx`
4.  - Assert that a policy report exists with a skipped result
