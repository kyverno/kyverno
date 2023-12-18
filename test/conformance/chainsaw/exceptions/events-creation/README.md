## Description

This test checks the events are generated properly for policyexceptions.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a policy exception for the cluster policy created above but for a specific namespace
1.  - Try to create a pod, expecting two events are created, one for the clusterpolicy, another is for policyexception

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/6469
