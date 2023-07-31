# ## Description

This test cleans up pods via a label assignment named `cleanup.kyverno.io/ttl: 10s`.

## Expected Behavior

The pod `test-pod` is cleaned up successfully after 10s.

## Reference Issue(s)
