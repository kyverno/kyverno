## Description

This test ensures that deletion of the source (upstream) resource used by a Policy `generate` rule with sync enabled using a clone declaration DOES cause deletion of downstream/cloned resources.

## Expected Behavior

After the source is deleted, the downstream resources should be deleted. If the downstream resource remains, the test fails. If the downstream resource is deleted, the test passes.

## Reference Issue(s)

N/A