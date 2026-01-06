# RSA Certificate Manager Test

This test verifies that Kyverno works correctly with RSA certificates managed by cert-manager through the helm chart integration.

## Purpose

This test ensures Kyverno can use cert-manager for TLS certificate management with RSA algorithm. The certificates are created automatically by the helm chart when `certManager.enabled` is set to `true`.

## Prerequisites

- cert-manager must be installed in the cluster before running this test

## How It Works

The test relies on the Kyverno helm chart's cert-manager integration:

1. **cert-manager installation**: The GitHub Action installs cert-manager before Kyverno
2. **Helm-based certificates**: The helm chart creates Certificate resources when `admissionController.certManager.enabled=true` and `cleanupController.certManager.enabled=true`
3. **Automatic provisioning**: cert-manager automatically provisions the TLS certificates with the correct secret names that Kyverno expects

## Test Steps

1. **Verify RSA certificates**: Check that the certificates use RSA algorithm
2. **Test webhook functionality**: Apply a test policy and verify the webhook works correctly

## Expected Result

This test should **pass** - RSA certificates are fully supported by Kyverno.

## Reference Issue(s)

- https://github.com/kyverno/kyverno/issues/14517
