## Description

This test creates a policy, and a resource.
The resource is expected to be rejected.
A `PolicyViolation` event should be created.

## Steps

1.  - Create a policy
    - Assert the policy becomes ready
1.  - Try to create a resource, expecting the creation to fail
1.  - Asset a `PolicyViolation` event is created
