# Ed25519 Self-Signed Certificates Test

This test verifies that Kyverno can generate and use self-signed Ed25519 certificates for TLS.

## Purpose

This test validates Kyverno's ability to generate its own self-signed certificates using the Ed25519 algorithm.

## What This Test Validates

1. **Ed25519 Certificate Generation**: Kyverno can generate self-signed CA and TLS certificates using Ed25519 keys
2. **Kyverno Startup**: The admission controller and cleanup controller start successfully with Ed25519 certificates
3. **Webhook Functionality**: The webhook works correctly with Ed25519 TLS certificates
4. **Certificate Verification**: The generated certificates are actually using Ed25519 (not RSA or ECDSA)

## Configuration

The test installs Kyverno with the following Helm values:

```yaml
admissionController:
  tlsKeyAlgorithm: Ed25519

cleanupController:
  tlsKeyAlgorithm: Ed25519
```

## Test Flow

1. Install Kyverno with `tlsKeyAlgorithm=Ed25519` for both admission and cleanup controllers
2. Wait for Kyverno to be ready
3. Verify the generated certificates use Ed25519 keys
4. Apply a test policy and verify webhook functionality

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/14548
