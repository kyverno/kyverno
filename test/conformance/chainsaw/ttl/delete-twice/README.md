# ## Description

This test cleans up pods via a label assignment named `cleanup.kyverno.io/ttl: 10s`.
Once deleted, the pod is created a second time and we expect to be deleted again.

## Expected Behavior

The pod `test-pod` is cleaned up successfully after 10s twice.

## Reference Issue(s)
