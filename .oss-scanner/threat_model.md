# Threat model

See [SECURITY.md](../SECURITY.md) for Kyverno's security policy and the [Kyverno Security Guide](https://kyverno.io/docs/guides/security/).

## What this project does and where untrusted input enters
Kyverno is a Kubernetes policy engine. Its controllers (admission, background, reports, cleanup) validate, mutate,
generate and clean up resources, and verify image signatures and attestations.

Untrusted input:
- **Policies and exceptions from namespaced users (tenants).** Tenants have less privilege than cluster admins. A
  namespaced policy must not read or affect anything outside its own namespace, including through `globalContext`,
  `apiCall`, `http` or `configMap` lookups.
- **AdmissionReview requests**, which carry arbitrary resource content.
- **Cluster resources** read by the background and reports controllers.
- **External data**: image registries, signatures and attestations (cosign, notary), and HTTP endpoints.
- **CLI files** passed to `kubectl-kyverno`: policies, resources and tests.

## Components that matter most / least
- Most: `pkg/engine/`, `pkg/cel/`, `pkg/webhooks/`, `pkg/validation/`, `pkg/image/` (image verification),
  `pkg/controllers/`, `cmd/background-controller/` (generate and mutate-existing write to cluster resources).
- Least: `test/`, `docs/`, `scripts/`, `hack/`, `charts/`, and generated code (`zz_generated.*`, `pkg/client/`).

## How to exercise it
- `/src/bin/kubectl-kyverno apply` and `test` run policies against resources offline. Fixtures are in `test/cli/`
  and `test/policy/`.
- Unit tests: `go test ./pkg/engine/... ./pkg/cel/...` (dependencies are already downloaded).
- For webhook code, write a unit test that builds an `AdmissionRequest`.

## How you rate severity
- **Critical**: a tenant policy or resource reads or changes cluster-wide data, escalates privileges or runs code in
  a controller; image signature verification bypass; remote code execution in any controller.
- **High**: a non-admin bypasses a policy or exception; cross-namespace data disclosure; SSRF to internal services
  from a tenant policy.
- **Medium**: one crafted policy or resource crashes or exhausts a controller (e.g. unbounded CPU or memory in CEL or
  JMESPath).
- **Low**: needs cluster-admin to exploit.

## Anything to leave alone
- Cluster admins, and anyone who can edit Kyverno's RBAC, ConfigMaps or webhook configuration, are trusted.
- Cluster-scoped policies that use `http` or `apiCall` are working as intended.
