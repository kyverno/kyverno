## Description

This test ensures that deletion of the source (upstream) resource used by a ClusterPolicy `generate` rule with sync enabled using a clone declaration does NOT cause deletion of downstream/cloned resources.

## Expected Behavior

After the source is deleted, the downstream resources should remain. If the downstream resource remains, the test passes. If the downstream resource is deleted, the test fails.

## Reference Issue(s)

N/A