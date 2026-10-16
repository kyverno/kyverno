# AGENTS.md — cmd/cli/kubectl-kyverno/processor

This package implements `PolicyProcessor`, which evaluates Kyverno policies (classic policies, validating admission policies, and CEL validating/generating/mutating policies) against an input resource or JSON payload.

## Shared Provider Lifecycle and Fallback Architecture

1. **Prebuilt Provider Sharing**:
   - Callers that evaluate multiple resources (such as `kubectl-kyverno apply`) can initialize and share evaluation components across resources:
     - `RESTMapper`: Prebuilt discovery REST mapper.
     - `ContextProvider`: Prebuilt context provider loaded once from `--context-file`.
     - `VpolProvider`, `VpolJSONProvider`: Compiled CEL validating policy providers.
     - `MpolProvider`: Compiled CEL mutating policy provider.
   - Passing these prebuilt instances into `PolicyProcessor` avoids per-resource disk I/O, context re-parsing, and CEL compilation overhead.

2. **Standalone Fallback Behavior**:
   - If any provider (`RESTMapper`, `ContextProvider`, `VpolProvider`, `VpolJSONProvider`, `MpolProvider`) is `nil`, `PolicyProcessor.ApplyPoliciesOnResource()` falls back to constructing it on-demand for that invocation.
   - This ensures standalone callers (e.g., `kubectl-kyverno test` or isolated unit tests) continue working without requiring prebuilt dependencies.

3. **CRD Lifecycle and Preloading**:
   - `CrdPreloaded`: When set to `true` by callers that eagerly load CRDs upfront, `ApplyPoliciesOnResource()` skips repeated per-resource calls to `loadCrds()`.
   - `CrdPaths`: Kept intact on `PolicyProcessor` so methods needing schema discovery (such as `openAPI()` constructing OpenAPI clients for `MutatingPolicy` type conversion) can access custom CRD definitions.

4. **Nil Safety**:
   - `getNamespaceResolver()` ensures that if `Variables` is uninitialized, a fallback no-op resolver `func(string) *corev1.Namespace { return nil }` is returned, preventing `nil` pointer panics in `vpolengine.Handle`.
