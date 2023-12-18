## Description

This test creates a mutate policy which adds labels to the newly created config maps.
An event is generated upon successful generation.

## Steps

1.  - Create a mutate policy
    - Assert the policy becomes ready
2.  Create a configmap.
3.  An event is created with a message indicating that the config map is successfully mutated.
