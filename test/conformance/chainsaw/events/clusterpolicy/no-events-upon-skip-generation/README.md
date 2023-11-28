## Description

This test creates a generate policy, and the trigger resource (namespace) `ns-1` which is excluded by the policy.
No events generated since the `ns-1`

## Steps

1.  - Create a generate policy
    - Assert the policy becomes ready
2.  Create the namespace.
3.  No events generated as the rule result is `skip`
