# ECDSA Certificate Manager Test

This test verifies that Kyverno works correctly with ECDSA certificates provided by cert-manager.

## Purpose

This test verifies that Kyverno can use externally managed ECDSA TLS certificates from cert-manager.

## Expected Result

This test should **fail** since Kyverno does not currently support ECDSA certificates.

The expected error message from the admission controller is:

```
failed to validate certificates error="x509: failed to parse private key (use ParseECPrivateKey instead for this key format)"
```

## Prerequisites

- cert-manager must be installed in the cluster before running this test

## Test Flow

1. **Assert cert-manager ready**: Verify cert-manager deployment is available
2. **Create namespace**: Create the `kyverno` namespace
3. **Create CA infrastructure**: 
   - Create a self-signed ClusterIssuer
   - Create ECDSA CA certificates with the exact secret names Kyverno expects
   - Create a CA Issuer using the CA certificate
4. **Create TLS certificates**: Create ECDSA certificates with Kyverno's expected secret names for:
   - `kyverno-svc` (admission controller)
   - `kyverno-cleanup-controller` (cleanup controller)
5. **Install Kyverno**: Install Kyverno using Helm (certificates already in place)
6. **Verify Kyverno ready**: Assert that Kyverno admission controller is ready
7. **Test webhook**: Apply a test policy to verify webhook functionality
