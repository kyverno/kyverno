## Description

This test updates the deployment with flag `--omit-events=PolicyApplied` set
Then it creates a policy, and a resource.
The resource is expected to be accepted.
A `PolicyApplied` event should be created.
Then it creates a respource that is expected to be rejected
A `PolicyViolation` event should not be emitted as the flag does not include that.

## Steps

1.  Update the deployment of admission controller to add this ar`--omit-events=PolicyApplied`.  
2.  - Create a policy
    - Assert the policy becomes ready
3.  - Create a resource,
4.  - Asset a `PolicyApplied` event is created
5.  Try creating a resource with a script that is expected to fail.
6.  Exit the script with `0` if it returns an error
