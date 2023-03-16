## Description

This is a basic test for the mutate existing capability which ensures that creating a triggering resource results in the correct mutation of a different resource.

## Expected Behavior

When the `dictionary-1` ConfigMap is created, this should result in the mutation of the Secret named `secret-1` within the same Namespace to add the label `foo: bar`. If the Secret is mutated so that the label `foo: bar` is present, the test passes. If not, the test fails.

## Reference Issue(s)

N/A