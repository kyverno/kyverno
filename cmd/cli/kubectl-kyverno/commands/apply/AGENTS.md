# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

This package implements the `kubectl-kyverno apply` CLI command. The command applies Kyverno policies to Kubernetes manifests, or to resources fetched from a cluster. Supported policy types are validating, mutating, generating, image-validating, deleting, and admission policies.

## Architecture and Execution Flow

1. **Policy & Resource Loading (`command.go`)**:
   - Parses flags and configuration (`ApplyCommandConfig`).
   - Loads policies into an `evaluator.PolicySet` (`loadPolicies`). Sources can be local files, git, or OCI. Invalid files are skipped, and legacy kinds are blocked.
   - Loads exceptions, variables, and resources.
   - When `--cluster` is set, creates a live dynamic/kube client (`dclient.Interface`). Otherwise it builds an in-memory fake client with `evaluator.NewFakeClient`.

2. **Policy Evaluation (`../../evaluator`)**:
   - `applyCommandHelper` handles flags, file I/O, and printing only. It calls `evaluator.Evaluate` once.
   - `evaluator.Evaluate` is the single evaluation path shared with `kyverno serve`. It runs the policy families in this fixed order:
     1. Admission policies: VAP/MAP, ValidatingPolicy, MutatingPolicy, and GeneratingPolicy, per resource and then per JSON payload.
     2. ImageValidatingPolicies.
     3. DeletingPolicies, on resources and then on JSON.
     4. HTTP and Envoy authz.
   - Evaluation code (processors, providers, ordering) belongs in `evaluator`, not here, so `apply` and `serve` cannot drift apart. `commands/serve/server_test.go` (`TestApplyParity`) runs both against the same fixtures and compares the results.

3. **Shared Namespace Resolver Invariant**:
   - CEL engine evaluations (ValidatingPolicies, MutatingPolicies, GeneratingPolicies) need a `NamespaceResolver` to evaluate `namespaceObject` and `namespaceSelector`.
   - `applyCommandHelper` creates the resolver **once** with `processor.GetNamespaceResolver(variables, dClient, c.Cluster)` and passes it in as `evaluator.Input.NamespaceResolver`. `Evaluate` uses that same instance for the admission, image-validating, and deleting passes.
   - **Critical invariant**: every policy pass in one `apply` run must share the same `NamespaceResolver` instance. This keeps namespace lookups cached in memory across passes, so they don't trigger repeated cluster queries.
