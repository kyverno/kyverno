# ## Description

This test cleans up pods instanteaously without any delay as the value of the label is `cleanup.kyverno.io/ttl: 2023-07-19T120000Z`  the timestamp is mentioned in past.

## Expected Behavior

The pod `test-pod` is cleaned up instantaneously.

The pod `test-pod-2` is cleaned up instantaneously when the label is updated to `cleanup.kyverno.io/ttl: 2023-07-19T120000Z` the timestamp is mentioned in past.

## Reference Issue(s)

- [8242](https://github.com/kyverno/kyverno/issues/8242): `test-pod` might never be created, so the assert could fail.
