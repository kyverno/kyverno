# RSA Certificate Manager Test

This test verifies that Kyverno works correctly with RSA certificates provided by cert-manager.

## Purpose

This test serves as a baseline to ensure Kyverno can use externally managed TLS certificates from cert-manager. RSA is the default and currently supported algorithm.

## Prerequisites

- cert-manager must be installed in the cluster before running this test

## Steps

1. **Assert cert-manager ready**: Verify cert-manager deployment is available
2. **Create namespace**: Create the `kyverno` namespace
3. **Create CA infrastructure**: 
   - Create a self-signed ClusterIssuer
   - Create CA certificates with the exact secret names Kyverno expects
   - Create a CA Issuer using the CA certificate
4. **Create TLS certificates**: Create certificates with Kyverno's expected secret names for:
   - `kyverno-svc` (admission controller)
   - `kyverno-cleanup-controller` (cleanup controller)
5. **Install Kyverno**: Install Kyverno using Helm (certificates already in place)
6. **Verify Kyverno ready**: Assert that Kyverno admission controller is ready
7. **Test webhook**: Apply a test policy to verify webhook functionality

## Expected Result

This test should **pass** with the current Kyverno implementation, as RSA certificates are fully supported.

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/14517
