## Description

This is a basic test for the mutate existing capability which ensures that creating of a Kyverno ClusterPolicy causes immediate mutation of downstream targets by setting `mutateExistingOnPolicyUpdate: true`.

## Expected Behavior

When the ClusterPolicy is created, at that time it should mutate the `test-secret-3` Secret in the `staging-3` Namespace to add a label with key `foo` the value of which should be the name of the defined triggering resource, `dictionary-3`. If this mutation is done, the test passes. If not, the test fails.

## Reference Issue(s)

N/A