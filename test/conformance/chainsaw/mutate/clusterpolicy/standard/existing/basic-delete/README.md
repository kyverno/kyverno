## Description

This is a basic test for the mutate existing capability which ensures that specifically deleting a triggering resource, via a precondition, results in the correct mutation of a different resource.

## Expected Behavior

When the `dictionary-2` ConfigMap is deleted, this should result in the mutation of the Secret named `test-secret-2` within the same Namespace to add the label `foo` with value set to the name or `dictionary-2` in this case. If the Secret is mutated so that the label `foo: dictionary-2` is present, the test passes. If not, the test fails.

## Reference Issue(s)

N/A