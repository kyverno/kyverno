# AGENTS.md — cmd/cli/kubectl-kyverno/commands/oci/internal/bundle

This package implements the packaging, assembly, validation, writing, and reading of Kyverno CEL Policy OCI bundles conforming to `docs/dev/oci/bundle-spec.md` (format v1.0.0, shipping in Kyverno 1.20).

## Writer vs. Reader Validation Split

Validation is partitioned into two distinct entry points in `validate.go` to balance authoring rigor against pull-path compatibility:

1. **Writer Validation (`Validate` / `ValidateWithOptions`)**:
   - Invoked on the bundle creation and push paths (`kyverno oci push`, `kyverno oci validate`).
   - Runs with `strict=true` by default.
   - Enforces author constraints: requires in-bundle resolution for all `PolicyException` policy references (bundle-spec.md section 6), detects duplicate resource identities, ensures matchConstraints selectors are valid, and validates that all CEL expressions compile cleanly.
   - Strict mode emits warnings for expired exceptions (authoring check).

2. **Reader Validation (`ValidateForRead`)**:
   - Invoked on the bundle consumption paths (`kyverno oci pull`, `kyverno test oci://...`, `kyverno apply oci://...`). `kyverno oci inspect` instead uses `ReadMetadata` and does not fetch or validate the content layer.
   - Runs with `strict=false`.
   - Verifies bundle integrity and schema compliance without enforcing authoring constraints or strict reference resolution that would prevent pulling valid historical bundles (e.g. expired exceptions are ignored, and cross-bundle policy references are permitted).

## Mode-Specific CEL Compiler Requirements

When verifying policy CEL expressions in `validateResults`:

- **Kubernetes Admission Mode (`ValidatingPolicy`, `NamespacedValidatingPolicy`)**:
  - Compiled using `vpolcompiler.NewCompiler()`.
  - Exposes standard Kubernetes admission variables (`object`, `oldObject`, `request`, `authorizer`).

- **Envoy Authorization Mode (`ValidatingPolicy` with `spec.evaluation.mode: Envoy`)**:
  - MUST be compiled using `authzcompiler.NewCompiler[dynamic.Interface, *authv3.CheckRequest, *authv3.CheckResponse](nil)`.
  - Exposes the `envoy.*` CEL extension library (e.g., `envoy.Allowed().Response()`).

- **HTTP Authorization Mode (`ValidatingPolicy` with `spec.evaluation.mode: HTTP`)**:
  - MUST be compiled using `authzcompiler.NewCompiler[dynamic.Interface, *authzhttp.CheckRequest, *authzhttp.CheckResponse](nil)`.
  - Exposes the `http.*` CEL extension library (e.g., `http.Allowed().Response()`).

> **Critical Invariant**: Never route `Envoy` or `HTTP` mode policies through `vpolcompiler.NewCompiler()`. The Kubernetes admission CEL environment does not declare `envoy` or `http` identifiers and will reject valid authorization policies with undeclared reference errors.

## Other Policy Families

- **`MutatingPolicies`**: Compiled via `mpolcompiler.NewCompiler()`.
- **`GeneratingPolicies`**: Compiled via `gpolcompiler.NewCompiler()`.
- **`DeletingPolicies`**: Compiled via `dpolcompiler.NewCompiler()`.
- **`PolicyExceptions`**: Match conditions compiled via `celcompiler.CompilePolicyExceptionMatchConditions()`.
