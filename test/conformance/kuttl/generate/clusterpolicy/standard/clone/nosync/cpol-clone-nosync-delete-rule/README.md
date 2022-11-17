## Description

This test ensures that deletion of a rule within a policy containing multiple rules, with a generate rule using clone and no-sync, does NOT cause the downstream resource to be deleted.

## Expected Behavior

Once the rule is deleted, the downstream resource is expected to remain. If it does remain, the test passes. If it gets deleted, the test fails.

## Reference Issue(s)

N/A