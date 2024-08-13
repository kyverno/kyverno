## Description

This is a basic test for the mutate existing capability, using a JSON patch, which ensures that creating a triggering resource results in the correct mutation of a different resource.

## Expected Behavior

When the `dictionary-4` ConfigMap is created, this should result in the mutation of the Secret named `test-secret-4` within the same Namespace to add the label `env` with value set to the name of the triggering resource's Namespace, `staging-4`. If the Secret is mutated so that the label `env: staging-4` is present, the test passes. If not, the test fails.

## Reference Issue(s)

N/A