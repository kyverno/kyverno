## Description

This test checks to ensure that a generate rule in a Policy (Namespaced) with a data declaration and NO synchronization, when a rule within the Policy having two rules is deleted does NOT cause any of the generated resources corresponding to that removed rule to be deleted.

## Expected Behavior

If both generated resources remain after deletion of the rule, the test passes. If either one is deleted, the test fails.

## Reference Issue(s)

N/A