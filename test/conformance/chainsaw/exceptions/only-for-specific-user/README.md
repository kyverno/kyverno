## Description

This test creates a policy, a policy exception and tries to create a configmap that violates the policy.
The exception should not apply as it is for a specific user and the configmap creation is expected to be rejected.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above but for a specific user
1.  - Try to create a confimap, expecting the creation to fail

## Reference Issue(s)

5930
