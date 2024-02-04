## Description

This test creates the following global context entries:
1. A valid global context entry.
2. A context entry with both `kubernetesResource` and `apiCall`.
3. A context entry with neither `kubernetesResource` nor `apiCall`.

## Expected Behavior

1st global context entry should get created, 2nd and 3rd entries should return an error.
