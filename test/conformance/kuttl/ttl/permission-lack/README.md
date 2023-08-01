# ## Description

This test must not be able to clean up config map as the service account mounted does not have required permission to cleanup the config map via the `cleanup.kyverno.io/ttl: 10s` label assignment.

## Expected Behavior

The pod `test-cm` is not cleaned up successfully after 10s.

## Reference Issue(s)
