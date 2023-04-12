## Description

This test ensures that modification of the source (upstream) resource used by a Policy `generate` rule with sync enabled using a clone declaration causes those changes to be synced/propagated downstream.

## Expected Behavior

After the source is modified, the downstream resources should be synced to reflect those modifications. If the downstream resource reflects the changes made to the source, the test passes. If the downstream resource remains unsynced, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5277