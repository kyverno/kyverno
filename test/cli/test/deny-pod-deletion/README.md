## Description

This test checks to ensure that a pod cannot be deleted, but can be created or updated. The test ensures that deletion operations can be specified in `validate.deny` expressions and not just `operations[]` under a `match` block.

## Expected Behavior

If the downstream resource is deleted, the test fails. If it remains, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8644
