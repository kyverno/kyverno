## Description

This test verifies that a pod creation request is validated by performing an HTTP GET call via Kyverno's cel lib `http.get` .

## Expected Behavior

The `good-pod` creation should succeed only if the HTTP GET request returns the expected response.
