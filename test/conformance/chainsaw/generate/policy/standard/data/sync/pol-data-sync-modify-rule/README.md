## Description

This is a generate test to ensure a generate Policy using a data declaration with sync enabled and modifying the policy/rule propagates those changes to a downstream ConfigMap.

## Expected Behavior

The downstream (generated) resource is expected to be synced from the corresponding rule within a Policy is modified. If it is not sync, the test fails. If it is synced, the test passes.

## Reference Issue(s)

N/A
