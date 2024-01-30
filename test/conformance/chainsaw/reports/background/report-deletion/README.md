## Description

This test creates a policy and a pod, it then expects a background scan report to be created for the pod.
When the policy is deleted, the background scan report should also be deleted.

## Steps

1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Create a pod
1.  - Assert a policy report is created for the pod and contains the right summary
1.  - Delete the policy
    - Assert the policy report is deleted for the pod
