## Description

This test checks to ensure that deletion of a rule in a Policy (Namespaced) generate rule, data declaration, with sync enabled, results in the downstream resource's deletion.

## Expected Behavior

The downstream (generated) resource is expected to be deleted if the corresponding rule within a Policy is deleted. If it is not deleted, the test fails. If it is deleted, the test passes.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/5744
