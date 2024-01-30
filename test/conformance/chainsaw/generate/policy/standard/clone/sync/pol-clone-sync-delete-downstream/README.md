## Description

This test ensures that deletion of a downstream resource created by a Policy `generate` rule with sync enabled using a clone declaration causes it to be regenerated. If it is not regenerated, the test fails.

## Expected Behavior

The downstream resource, upon deletion, is expected to be recreated/recloned from the source resource.

## Reference Issue(s)

N/A