## Description

This test checks to ensure that a generate rule with a data declaration and NO synchronization, when a rule within a policy is changed (under the data object) that this does NOT cause the downstream resource to be synced.

## Expected Behavior

If the downstream resource is NOT modified from its initial generation, the test passes. If the downstream resource is synced from the changes made to the rule, the test fails.

## Reference Issue(s)

N/A