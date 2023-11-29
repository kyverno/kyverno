## Description

This is test migrated from e2e which roughly tests that mutations are successful when the value of key being mutated contains both a context variable as well as a context variable plus additional JMESPath filtering in that variable reference. The test migrated here to kuttl represents a condensed version of the original test to eliminate minor redundancy.

## Expected Behavior

The mutated ConfigMap should have a label written to it `kyverno.key/copy-me: sample-value`. If this is so, the test passes. If it is not, the test fails.

## Reference Issue(s)

N/A
