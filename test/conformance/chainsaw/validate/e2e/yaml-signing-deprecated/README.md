## Description

This test is migrated from e2e. It tests basic YAML manifest signature validation functionality.

## Expected Behavior

The `test-deployment` (defined in `bad.yaml`) should fail because it matches the policy conditions yet has not been signed while the `test-deployment` (defined in `02-good-deployment.yaml`) should pass because it also matches yet has been signed and the signature is valid according to the public key defined in the policy.

## Reference Issue(s)

N/A
