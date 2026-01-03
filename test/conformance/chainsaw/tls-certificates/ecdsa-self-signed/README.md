# ECDSA Self-Signed Certificates Test

This test verifies that Kyverno can generate and use self-signed ECDSA certificates for TLS.

## Purpose

This test validates Kyverno's ability to generate its own self-signed certificates using the ECDSA algorithm (P-256 curve).

## Configuration

The test installs Kyverno with the following Helm values:

```yaml
admissionController:
  tlsKeyAlgorithm: ECDSA

cleanupController:
  tlsKeyAlgorithm: ECDSA
```

## Test Flow

1. Install Kyverno with `tlsKeyAlgorithm=ECDSA` for both admission and cleanup controllers
2. Wait for Kyverno to be ready
3. Verify the generated certificates use ECDSA keys
4. Apply a test policy and verify webhook functionality

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/14548
