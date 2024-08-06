## Description

This test checks to ensure that when a standard generate policy with data type and sync enabled is used, modification of the generated/downstream resource causes Kyverno to sync the resource from the definition in the rule.

## Expected Behavior

If the resource is synced from the definition in the rule, the test passes. If it is not and remains in the modified state, the test fails.

## Reference Issue(s)

N/A