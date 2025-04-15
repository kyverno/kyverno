## Description

This test verifies that a pod creation request is validated by performing an HTTP POST call via Kyverno's cel lib `http.post` .

## Expected Behavior

The `good-pod` creation should succeed only if the HTTP POST request returns the expected response.
