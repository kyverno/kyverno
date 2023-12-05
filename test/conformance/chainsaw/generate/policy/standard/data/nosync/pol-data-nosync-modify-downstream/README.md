## Description

This test makes sure that a Policy (Namespaced) with a generate rule, data type, and sync NOT enabled, when the downstream (generated) resource is modified this does NOT result in those modifications being reverted based upon the definition stored in the rule.

## Expected Behavior

If the generated resource remains in the modified state, the test passes. If it is synced with the contents in the rule, the test fails.

## Reference Issue(s)

N/A