# Propagation Policy Tests

This folder contains tests for the `determinePropagationPolicy` function, which handles cleanup propagation policies based on resource annotations.

## Test Cases

1. **Foreground Policy**: Verifies that the resource is deleted after its dependents.
   - File: `foreground-policy.yaml`

2. **Background Policy**: Verifies that the resource is deleted immediately, and dependents are deleted asynchronously.
   - File: `background-policy.yaml`

3. **Orphan Policy**: Verifies that dependents are orphaned when the resource is deleted.
   - File: `orphan-policy.yaml`

4. **No Policy/Invalid Policy**: Verifies that no action is taken when the annotation is missing or unknown.
   - File: `no-annotation.yaml`
