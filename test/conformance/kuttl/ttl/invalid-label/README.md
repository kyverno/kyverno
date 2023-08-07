# ## Description

This test must not be able to clean up pod as the label assignment is invalid which will not be recognized by the controller in this case the label is named `cleanup.kyverno.io/ttl: 10ay`.

## Expected Behavior

The pod `test-pod` is not cleaned up successfully after 10s.

## Reference Issue(s)
