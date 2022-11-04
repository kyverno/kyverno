# Title

This test ensures that deletion of a downstream resource created by a ClusterPolicy `generate` rule with sync disabled using a clone declaration does NOT cause it to be regenerated. If the downstream resource is regenerated, the test fails. If it is not regenerated, the test succeeds.

### Tests a clone rule with sync not enabled that deleting a downstream resource shows it is not recreated.
### Because https://github.com/kyverno/kyverno/issues/4457 is not yet fixed for this type, the test will fail.
### Expected result: fail