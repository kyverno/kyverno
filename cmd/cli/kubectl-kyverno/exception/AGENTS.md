# AGENTS.md — cmd/cli/kubectl-kyverno/exception

This package provides policy exception loading and filtering for the Kyverno CLI (`kubectl-kyverno`), supporting both legacy `kyverno.io` PolicyExceptions and CEL-based `policies.kyverno.io` PolicyExceptions.

## Single Source of Truth for Exception GVKs

This package exports canonical GVK definitions and classification predicates that serve as the single source of truth across the CLI:

- **Definitions**: `ExceptionV2beta1`, `ExceptionV2`, `CELExceptionV1alpha1`, `CELExceptionV1beta1`, `CELExceptionV1`
- **Classification Predicates**:
  - `IsLegacyException(gvk)`: matches `kyverno.io/v2beta1` and `kyverno.io/v2` PolicyExceptions
  - `IsCELException(gvk)`: matches `policies.kyverno.io/v1alpha1`, `policies.kyverno.io/v1beta1`, and `policies.kyverno.io/v1` PolicyExceptions
  - `IsPolicyException(gvk)`: returns true for any recognized PolicyException

Both this package's loaders (`Load`, `SelectFrom`) and the general policy loader in `cmd/cli/kubectl-kyverno/policy` (`processDocumentItem`) consume these predicates. **Never duplicate version lists or enumerate individual CEL exception versions at call sites in loaders** — add any future versions to this package's definitions and predicates so all CLI loaders remain consistent.

## Target Struct Types and Conversions

- **Legacy exceptions** (`kyverno.io`) convert via `convert.To[kyvernov2.PolicyException]` into `LoaderResults.Exceptions` (`[]*kyvernov2.PolicyException`).
- **CEL exceptions** (`policies.kyverno.io`) convert via `convert.To[policiesv1beta1.PolicyException]` into `LoaderResults.CELExceptions` (`[]*policiesv1beta1.PolicyException`).
- Unstructured resources are converted strictly using `convert.To[...]` from `ext/resource/convert`, which respects schema validation and rejects unknown fields.

## Deprecation Blocking and Error Handling Semantics

- **`allowLegacyPolicies` Gate**: When `allowLegacyPolicies` is `false`, legacy `kyverno.io` exceptions are blocked with a migration hint via `pkgdeprecations.BuildKindError`. Deprecation warnings are emitted via `pkgdeprecations.BuildKindWarning`.
- **Multi-Document Scanning in `Load()`**: Scans through all YAML documents using `pendingErr` to remember earlier validation/unsupported errors while continuing to scan. A legacy deprecation block always takes priority and returns immediately; otherwise, `pendingErr` is returned so broken documents fail the load rather than being silently dropped.
- **Inline Extraction in `SelectFrom()`**: Extracts exceptions from slices of unstructured resources (e.g. `--exceptions-within-resources`/`--inline-exceptions`), using the same classification predicates and deprecation checks. A malformed exception records an error while still allowing valid sibling exceptions to be picked up.
