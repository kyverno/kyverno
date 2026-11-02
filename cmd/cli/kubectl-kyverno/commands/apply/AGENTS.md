# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

This package implements the `kubectl-kyverno apply` CLI command, which applies Kyverno policies (validating, mutating, generating, image-validating, deleting, and admission policies) to Kubernetes manifests or resources fetched from a cluster.

## Architecture and Execution Flow

1. **Policy & Resource Loading (`command.go`, `load.go`)**:
   - Parses flags and configurations (`ApplyCommandConfig`).
   - Loads policies, exceptions, variables, and resources from local files or git sources.
   - When `--cluster` is specified, initializes a live dynamic/kube client (`dclient.Interface`).

2. **Policy Evaluation Passes (`command.go`)**:
   - `applyCommandHelper` coordinates evaluation across multiple policy families in sequence:
     - Standard admission policies (`applyPolicies`): validating, mutating, generating, and validating/mutating admission policies.
     - Image-validating policies (`applyImageValidatingPolicies`).
     - Deleting policies (`applyDeletingPolicies`).
     - Authorization policies (HTTP/Envoy).

3. **Shared Namespace Resolver Invariant**:
   - CEL engine evaluations (such as ValidatingPolicies, MutatingPolicies, and GeneratingPolicies) require a `NamespaceResolver` to evaluate `namespaceObject` and `namespaceSelector`.
   - `nsResolver := processor.GetNamespaceResolver(variables, dClient, c.Cluster)` is instantiated **once** in `applyCommandHelper` and passed into `applyPolicies`, `applyImageValidatingPolicies`, and `applyDeletingPolicies`.
   - **Critical Invariant**: All policy passes within a single `apply` invocation must share the same `NamespaceResolver` instance so that namespace lookups cached in memory are retained across all evaluation passes and do not trigger redundant cluster queries.
