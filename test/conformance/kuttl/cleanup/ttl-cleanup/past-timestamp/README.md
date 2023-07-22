# ## Description

This test cleans up pods instanteaously without any delay as the value of the label is `kyverno.io/ttl: 2023-07-19T120000Z`  the timestamp is mentioned in past.

## Expected Behavior

The pod `test-pod` is cleaned up instantaneously.

## Reference Issue(s)
