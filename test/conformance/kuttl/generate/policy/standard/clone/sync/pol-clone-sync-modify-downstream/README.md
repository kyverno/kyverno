## Description

This test ensures that modification of the downstream (cloned) resource used by a Policy `generate` rule with sync enabled using a clone declaration causes those changes to be reverted and synchronized from the state of the upstream/source.

## Expected Behavior

After the downstream resource is modified, the changes should be reverted after synchronization occurs. If the downstream resource is synced with the state of the source resource, the test passes. If the downstream resource remains in a modified state, the test fails.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5100