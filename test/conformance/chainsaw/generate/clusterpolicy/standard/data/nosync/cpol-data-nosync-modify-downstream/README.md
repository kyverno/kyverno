## Description

This test checks to ensure that a generate rule with a data declaration and NO synchronization, when a downstream (generated) resource is modified this does NOT result in those modifications getting reverted based upon the definition in the rule.

## Expected Behavior

If the downstream resource is left in the modified state, the test passes. If the downstream resource is synced from the definition in the rule, the test fails.

## Reference Issue(s)

N/A