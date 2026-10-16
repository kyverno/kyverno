# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

This package implements the `kubectl-kyverno apply` CLI command, which applies Kyverno policies (classic policies, validating admission policies, and CEL validating/generating/mutating policies) against resources and JSON payloads.

## Shared Lifecycle and Evaluation Flow

1. **Policy and Manifest Loading**:
   - Parses policies, resources, variables, exceptions, and CRDs specified via command-line flags.
   - Normalizes flags and verifies mutual exclusions (e.g., `--crd-paths` vs `--kubeconfig`).

2. **Offline CRD Eager Loading**:
   - When `--crd-paths` is specified, CRDs are eagerly loaded once via `common.LoadCrdsFromPath` before building the REST mapper.
   - This ensures custom resource definitions and GVKs are registered in discovery and the schema cache before evaluation begins.
   - `PolicyProcessor.CrdPreloaded` is set to `true` so `ApplyPoliciesOnResource` skips redundant per-resource disk I/O and re-registration, while preserving `CrdPaths` for components that need schema access (such as OpenAPI type conversion in mutating policies).

3. **Conditional CEL Context and Provider Pre-Initialization**:
   - `utils.GetRESTMapper`, `processor.NewContextProvider`, and compiled CEL policy providers (`VpolProvider`, `VpolJSONProvider`, `MpolProvider`) are initialized once prior to resource evaluation loops only when CEL policies (`vpols`, `gpols`, `mpols`) are present.
   - Runs with classic policies only leave these fields `nil`, preserving zero setup overhead and preventing irrelevant `--context-file` errors.
   - Kubernetes-mode validating policies (`VpolProvider`) are compiled only when Kubernetes resources are being evaluated (`len(resources) > 0`), avoiding unnecessary compilation and errors for JSON-only evaluations.
   - When `--continue-on-fail` is enabled, provider compilation failures are deferred to per-resource evaluation by leaving the provider field `nil`, preserving continue-on-fail semantics.

4. **Resource Evaluation Loops**:
   - Evaluates input resources and JSON payloads by passing pre-initialized shared components into `processor.PolicyProcessor`.
   - `PolicyProcessor` reuses the prebuilt providers when supplied, while retaining standalone on-demand fallback initialization for backward compatibility.
