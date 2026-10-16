# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

This package implements the `kubectl-kyverno apply` CLI command, which applies Kyverno policies (classic policies, validating admission policies, and CEL validating/generating/mutating policies) against resources and JSON payloads.

## Shared Lifecycle and Evaluation Flow

1. **Policy and Manifest Loading**:
   - Parses policies, resources, variables, exceptions, and CRDs specified via command-line flags.
   - Normalizes flags and verifies mutual exclusions (e.g., `--crd-paths` vs `--kubeconfig`).

2. **Offline CRD Eager Loading**:
   - When `--crd-paths` is specified, CRDs are eagerly loaded once via `common.LoadCrdsFromPath` before building the REST mapper.
   - This ensures custom resource definitions and GVKs are registered in discovery and the schema cache before evaluation begins.
   - `CrdPaths` is omitted from the per-resource `PolicyProcessor` initializations in `applyPolicies` to avoid re-reading and re-parsing CRDs from disk for each resource.

3. **Conditional CEL Context and Provider Pre-Initialization**:
   - `utils.GetRESTMapper`, `processor.NewContextProvider`, and compiled CEL policy providers (`VpolProvider`, `VpolJSONProvider`, `MpolProvider`) are initialized once prior to resource evaluation loops only when CEL policies (`vpols`, `gpols`, `mpols`) are present.
   - Runs with classic policies only leave these fields `nil`, preserving zero setup overhead and preventing irrelevant `--context-file` errors.

4. **Resource Evaluation Loops**:
   - Evaluates input resources and JSON payloads by passing pre-initialized shared components into `processor.PolicyProcessor`.
   - `PolicyProcessor` reuses the prebuilt providers when supplied, while retaining standalone on-demand fallback initialization for backward compatibility.
